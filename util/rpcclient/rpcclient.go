package rpcclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	flag "github.com/spf13/pflag"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/offchainlabs/nitro/util/signature"
)

type ClientConfig struct {
	URLs                      []string      `json:"urls,omitempty" koanf:"urls"`
	URL                       string        `json:"url,omitempty" koanf:"url"` // For backward compatibility
	JWTSecret                 string        `json:"jwtsecret,omitempty" koanf:"jwtsecret"`
	Timeout                   time.Duration `json:"timeout,omitempty" koanf:"timeout" reload:"hot"`
	Retries                   uint          `json:"retries,omitempty" koanf:"retries" reload:"hot"`
	ConnectionWait            time.Duration `json:"connection-wait,omitempty" koanf:"connection-wait"`
	ArgLogLimit               uint          `json:"arg-log-limit,omitempty" koanf:"arg-log-limit" reload:"hot"`
	RetryErrors               string        `json:"retry-errors,omitempty" koanf:"retry-errors" reload:"hot"`
	RetryDelay                time.Duration `json:"retry-delay,omitempty" koanf:"retry-delay"`
	WebsocketMessageSizeLimit int64         `json:"websocket-message-size-limit,omitempty" koanf:"websocket-message-size-limit"`
	HealthCheckInterval       time.Duration `json:"health-check-interval,omitempty" koanf:"health-check-interval"`
	HealthCheckTimeout        time.Duration `json:"health-check-timeout,omitempty" koanf:"health-check-timeout"`
	MaxConsecutiveFailures    int           `json:"max-consecutive-failures,omitempty" koanf:"max-consecutive-failures"`

	retryErrors         *regexp.Regexp
	currentURLIndex     int
	consecutiveFailures int
	lastHealthCheck     time.Time
}

func (c *ClientConfig) Validate() error {
	if c.RetryErrors == "" {
		c.retryErrors = nil
		return nil
	}
	var err error
	c.retryErrors, err = regexp.Compile(c.RetryErrors)
	return err
}

func (c *ClientConfig) UnmarshalJSON(data []byte) error {
	// Use DefaultClientConfig for default values when unmarshalling JSON
	*c = DefaultClientConfig
	type clientConfigWithoutCustomUnmarshal ClientConfig
	return json.Unmarshal(data, (*clientConfigWithoutCustomUnmarshal)(c))
}

type ClientConfigFetcher func() *ClientConfig

var TestClientConfig = ClientConfig{
	URL:                       "self",
	JWTSecret:                 "",
	WebsocketMessageSizeLimit: 256 * 1024 * 1024,
}

var DefaultClientConfig = ClientConfig{
	URLs:                      []string{"self-auth"},
	URL:                       "self-auth", // For backward compatibility
	JWTSecret:                 "",
	Retries:                   3,
	RetryErrors:               "websocket: close.*|dial tcp .*|.*i/o timeout|.*connection reset by peer|.*connection refused",
	ArgLogLimit:               2048,
	WebsocketMessageSizeLimit: 256 * 1024 * 1024,
	HealthCheckInterval:       30 * time.Second,
	HealthCheckTimeout:        5 * time.Second,
	MaxConsecutiveFailures:    3,
}

func RPCClientAddOptions(prefix string, f *flag.FlagSet, defaultConfig *ClientConfig) {
	f.StringSlice(prefix+".urls", defaultConfig.URLs, "list of RPC URLs to use with failover")
	f.String(prefix+".url", defaultConfig.URL, "single RPC URL (for backward compatibility)")
	f.String(prefix+".jwtsecret", defaultConfig.JWTSecret, "path to file with jwtsecret for validation - ignored if url is self or self-auth")
	f.Duration(prefix+".connection-wait", defaultConfig.ConnectionWait, "how long to wait for initial connection")
	f.Duration(prefix+".timeout", defaultConfig.Timeout, "per-response timeout (0-disabled)")
	f.Uint(prefix+".arg-log-limit", defaultConfig.ArgLogLimit, "limit size of arguments in log entries")
	f.Uint(prefix+".retries", defaultConfig.Retries, "number of retries in case of failure(0 mean one attempt)")
	f.String(prefix+".retry-errors", defaultConfig.RetryErrors, "Errors matching this regular expression are automatically retried")
	f.Duration(prefix+".retry-delay", defaultConfig.RetryDelay, "delay between retries")
	f.Int64(prefix+".websocket-message-size-limit", defaultConfig.WebsocketMessageSizeLimit, "websocket message size limit used by the RPC client. 0 means no limit")
	f.Duration(prefix+".health-check-interval", defaultConfig.HealthCheckInterval, "interval between health checks")
	f.Duration(prefix+".health-check-timeout", defaultConfig.HealthCheckTimeout, "timeout for health check requests")
	f.Int(prefix+".max-consecutive-failures", defaultConfig.MaxConsecutiveFailures, "maximum number of consecutive failures before switching to next URL")
}

type RpcClient struct {
	config    ClientConfigFetcher
	client    *rpc.Client
	autoStack *node.Node
	logId     atomic.Uint64

	healthCheckCtx    context.Context
	healthCheckCancel context.CancelFunc
}

func NewRpcClient(config ClientConfigFetcher, stack *node.Node) *RpcClient {
	return &RpcClient{
		config:    config,
		autoStack: stack,
	}
}

func (c *RpcClient) Close() {
	if c.healthCheckCancel != nil {
		c.healthCheckCancel()
	}
	if c.client != nil {
		c.client.Close()
	}
}

type limitedMarshal struct {
	limit uint
	value any
}

func (m limitedMarshal) String() string {
	marshalled, err := json.Marshal(m.value)
	var str string
	if err != nil {
		str = "\"CANNOT MARSHALL: " + err.Error() + "\""
	} else {
		str = string(marshalled)
	}
	// #nosec G115
	limit := int(m.limit)
	if m.limit <= 0 || len(str) <= limit {
		return str
	}
	prefix := str[:m.limit/2-1]
	postfix := str[len(str)-limit/2+1:]
	return fmt.Sprintf("%v..%v", prefix, postfix)
}

type limitedArgumentsMarshal struct {
	limit uint
	args  []any
}

func (m limitedArgumentsMarshal) String() string {
	res := "["
	for i, arg := range m.args {
		res += limitedMarshal{m.limit, arg}.String()
		if i < len(m.args)-1 {
			res += ", "
		}
	}
	res += "]"
	return res
}

var blobTxUnderpricedRegexp = regexp.MustCompile(`replacement transaction underpriced: new tx gas fee cap (\d*) <= (\d*) queued`)

// IsAlreadyKnownError returns true if the error appears to be an "already known" error.
// This check is based on the error's string form and is not precise.
func IsAlreadyKnownError(err error) bool {
	s := err.Error()
	if strings.Contains(s, "already known") {
		return true
	}
	// go-ethereum returns "replacement transaction underpriced" instead of "already known" for blob txs.
	// This is fixed in https://github.com/ethereum/go-ethereum/pull/29210
	// TODO: Once a new geth release is out with this fix, we can remove this check.
	matches := blobTxUnderpricedRegexp.FindSubmatch([]byte(s))
	if len(matches) == 3 {
		return string(matches[1]) == string(matches[2])
	}
	return false
}

func (c *RpcClient) CallContext(ctx_in context.Context, result interface{}, method string, args ...interface{}) error {
	if c.client == nil {
		return errors.New("not connected")
	}
	logId := c.logId.Add(1)

	// Log the current RPC URL being used
	currentURL := c.config().GetCurrentURL()
	log.Trace("Making RPC call",
		"method", method,
		"logId", logId,
		"rpc_url", currentURL,
		"args", limitedArgumentsMarshal{c.config().ArgLogLimit, args})

	var err error
	for i := uint(0); i < c.config().Retries+1; i++ {
		retryDelay := c.config().RetryDelay
		if i > 0 && retryDelay > 0 {
			select {
			case <-ctx_in.Done():
				return ctx_in.Err()
			case <-time.After(retryDelay):
			}
		}
		if ctx_in.Err() != nil {
			return ctx_in.Err()
		}
		var ctx context.Context
		var cancelCtx context.CancelFunc
		timeout := c.config().Timeout
		if timeout > 0 {
			ctx, cancelCtx = context.WithTimeout(ctx_in, timeout)
		} else {
			ctx, cancelCtx = context.WithCancel(ctx_in)
		}
		err = c.client.CallContext(ctx, result, method, args...)

		cancelCtx()
		logger := log.Trace
		limit := c.config().ArgLogLimit
		if err != nil && !IsAlreadyKnownError(err) {
			logger = log.Info
		}
		logEntry := []interface{}{
			"method", method,
			"logId", logId,
			"rpc_url", currentURL,
			"err", err,
			"result", limitedMarshal{limit, result},
			"attempt", i,
			"args", limitedArgumentsMarshal{limit, args},
		}
		var dataErr rpc.DataError
		if errors.As(err, &dataErr) {
			logEntry = append(logEntry, "errorData", limitedMarshal{limit, dataErr.ErrorData()})
		}
		logger("rpc response", logEntry...)
		if err == nil {
			return nil
		}
		if errors.Is(err, context.DeadlineExceeded) {
			continue
		}
		retryErrs := c.config().retryErrors
		if retryErrs != nil && retryErrs.MatchString(err.Error()) {
			continue
		}
		return err
	}
	return err
}

func (c *RpcClient) BatchCallContext(ctx context.Context, b []rpc.BatchElem) error {
	currentURL := c.config().GetCurrentURL()
	err := c.client.BatchCallContext(ctx, b)
	if err != nil {
		log.Error("Batch RPC call failed",
			"rpc_url", currentURL,
			"error", err)
	}
	return err
}

func (c *RpcClient) EthSubscribe(ctx context.Context, channel interface{}, args ...interface{}) (*rpc.ClientSubscription, error) {
	currentURL := c.config().GetCurrentURL()
	sub, err := c.client.EthSubscribe(ctx, channel, args...)
	if err != nil {
		log.Error("Failed to create subscription",
			"rpc_url", currentURL,
			"error", err)
	}
	return sub, err
}

func (c *RpcClient) Start(ctx_in context.Context) error {
	config := c.config()
	if config == nil {
		return errors.New("no config provided")
	}

	// Handle backward compatibility
	if config.URL != "" && len(config.URLs) == 0 {
		config.URLs = []string{config.URL}
	}

	if len(config.URLs) == 0 {
		return errors.New("no RPC URLs provided")
	}

	// Try to connect to the first URL
	err := c.connect(ctx_in)
	if err != nil {
		return fmt.Errorf("failed to connect to initial RPC URL: %w", err)
	}

	// Start health check routine
	c.healthCheckCtx, c.healthCheckCancel = context.WithCancel(ctx_in)
	go c.healthCheckRoutine()

	return nil
}

func (c *RpcClient) healthCheckRoutine() {
	config := c.config()
	if config == nil {
		return
	}

	ticker := time.NewTicker(config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.healthCheckCtx.Done():
			return
		case <-ticker.C:
			// config.NextURL()
			// config.consecutiveFailures = 0

			// if err := c.connect(c.healthCheckCtx); err != nil {
			// 	log.Error("Failed to connect to next URL", "error", err)
			// }

			if err := c.performHealthCheck(); err != nil {
				log.Warn("Health check failed", "error", err, "url", config.GetCurrentURL())
				config.consecutiveFailures++

				if config.consecutiveFailures >= config.MaxConsecutiveFailures {
					log.Info("Switching to next RPC URL due to consecutive failures",
						"failures", config.consecutiveFailures,
						"current_url", config.GetCurrentURL())

					config.NextURL()
					config.consecutiveFailures = 0

					if err := c.connect(c.healthCheckCtx); err != nil {
						log.Error("Failed to connect to next URL", "error", err)
					}
				}
			} else {
				config.consecutiveFailures = 0
			}
		}
	}
}

func (c *RpcClient) performHealthCheck() error {
	if c.client == nil {
		return errors.New("not connected")
	}

	currentURL := c.config().GetCurrentURL()
	log.Info("Making RPC health check call",
		"rpc_url", currentURL)

	ctx, cancel := context.WithTimeout(c.healthCheckCtx, c.config().HealthCheckTimeout)
	defer cancel()

	var result string
	err := c.client.CallContext(ctx, &result, "eth_blockNumber")
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	return nil
}

func (c *RpcClient) connect(ctx_in context.Context) error {
	config := c.config()
	if config == nil {
		return errors.New("no config provided")
	}

	url := config.GetCurrentURL()
	log.Info("Attempting to connect to RPC", "url", url)

	jwtPath := config.JWTSecret
	if url == "self" {
		if c.autoStack == nil {
			return errors.New("self not supported for this connection")
		}
		url = c.autoStack.WSEndpoint()
		jwtPath = ""
	} else if url == "self-auth" {
		if c.autoStack == nil {
			return errors.New("self-auth not supported for this connection")
		}
		url = c.autoStack.WSAuthEndpoint()
		jwtPath = c.autoStack.JWTPath()
	} else if url == "" {
		return errors.New("no url provided for this connection")
	}
	var jwt *common.Hash
	if jwtPath != "" {
		var err error
		jwt, err = signature.LoadSigningKey(jwtPath)
		if err != nil {
			return err
		}
	}
	connTimeout := time.After(config.ConnectionWait)
	for {
		var ctx context.Context
		var cancelCtx context.CancelFunc
		timeout := config.Timeout
		if timeout > 0 {
			ctx, cancelCtx = context.WithTimeout(ctx_in, timeout)
		} else {
			ctx, cancelCtx = context.WithCancel(ctx_in)
		}
		var err error
		var client *rpc.Client
		if jwt == nil {
			client, err = rpc.DialOptions(ctx, url, rpc.WithWebsocketMessageSizeLimit(config.WebsocketMessageSizeLimit))
		} else {
			client, err = rpc.DialOptions(ctx, url, rpc.WithHTTPAuth(node.NewJWTAuth([32]byte(*jwt))), rpc.WithWebsocketMessageSizeLimit(config.WebsocketMessageSizeLimit))
		}
		cancelCtx()
		if err == nil {
			c.client = client
			log.Info("Successfully connected to RPC", "url", url)
			return nil
		}
		if strings.Contains(err.Error(), "parse") ||
			strings.Contains(err.Error(), "malformed") {
			return fmt.Errorf("%w: url %s", err, url)
		}
		log.Warn("Failed to connect to RPC, retrying", "url", url, "error", err)
		select {
		case <-connTimeout:
			return fmt.Errorf("timeout trying to connect lastError: %w", err)
		case <-time.After(time.Second):
		}
	}
}

// GetCurrentURL returns the current URL from the list of URLs
func (c *ClientConfig) GetCurrentURL() string {
	if len(c.URLs) == 0 {
		return ""
	}
	return c.URLs[c.currentURLIndex]
}

// NextURL advances to the next URL in the list
func (c *ClientConfig) NextURL() {
	if len(c.URLs) > 1 {
		c.currentURLIndex = (c.currentURLIndex + 1) % len(c.URLs)
	}
}
