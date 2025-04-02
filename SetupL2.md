==========================================

# Deploy Rollup Creator & TokenBridgeCreator in Layer 1

Docs link: https://github.com/OffchainLabs/token-bridge-contracts/blob/v1.2.3/docs/deployment.md
Về cơ bản là deploy các smartcontract đã đc team nitro code sẵn, nhưng phải chỉnh sửa 1 chút để phù hợp.

VD để deploy trên BSC testnet thì disable ArbitrumChecker đi
`library ArbitrumChecker {
    function runningOnArbitrum() internal pure returns (bool) {
        // (bool ok, bytes memory data) = address(100).staticcall(
        //     abi.encodeWithSelector(ArbSys.arbOSVersion.selector)
        // );
        // return ok && data.length == 32;
        return false;
    }
}`
`const Reader4844Bytecode = "606f600d600039606f6000f3fe3415600957600080fd5b60003560e01c63e83a2d828114602857631f6d6ef78114606457600080fd5b60005b600115605057804980603c57506050565b80604060208402015260018201915050602b565b602060005280602052604060208202016000f35b4a8060005260206000f3";
`

Và rồi tiếp đó là chạy để deploy thôi, nhớ là chỉnh gasLimit xuống nếu ko là hàm sẽ tự động lấy 100gWei => tốn kém.

` const contract: Contract = await connectedFactory.deploy(...deploymentArgs, {
    // gasLimit: 10000000,
    gasPrice: 3000001000, // 3.000001 gWei
  })
 `

`yarn run deploy-factory --network bscTestnet`

# Sau đó là sang deploy TokenBridgeCreator

Ghi đè gasPrice trong atomixTokenBidgeDeployer, đoạn này hơi phiền là phải thay hết cho cả 19 cái contract luôn
nhớ cài forge chạy `forge install` trước vì script này chạy bằng forge chứ ko phải hardhat, cài ENV verify key đầy đủ.

`yarn run deploy:token-bridge-creator`

=> forge verify mượt và nhanh hơn hardhat

# Sau đó là deploy rollup

Cũng nhớ chỉnh gasPrice để giảm phí gas
`
import { arbitrumSepolia, bscTestnet } from 'viem/chains'

export const updatedBscTestnet = {
...bscTestnet,
contracts: {
...bscTestnet.contracts,
rollupCreator: {
address: '0xCAD3ae0523811b9d2eb3B26E12427ccE9D671c9A' as `0x${string}`,
},
tokenBridgeCreator: {
address: '0xB0C6B849cdd80Ee5D925C0454F151B75FC23CC8E' as `0x${string}`,
},
},
fees: {
defaultPriorityFee: 1500002000n,
},
}
`

`createRollup.ts`
```
import { Chain, createPublicClient, http, Address } from 'viem'
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts'
import { arbitrumSepolia, bscTestnet } from 'viem/chains'
import {
  createRollupPrepareDeploymentParamsConfig,
  prepareChainConfig,
  createRollupEnoughCustomFeeTokenAllowance,
  createRollupPrepareCustomFeeTokenApprovalTransactionRequest,
  createRollupPrepareTransactionRequest,
  createRollupPrepareTransactionReceipt,
  registerCustomParentChain,
} from '@arbitrum/orbit-sdk'
import { sanitizePrivateKey, generateChainId } from '@arbitrum/orbit-sdk/utils'
import { config } from 'dotenv'
import { updatedBscTestnet } from './utils'

config()

function withFallbackPrivateKey(privateKey: string | undefined): `0x${string}` {
  if (typeof privateKey === 'undefined' || privateKey === '') {
    return generatePrivateKey()
  }

  return sanitizePrivateKey(privateKey)
}

function getBlockExplorerUrl(chain: Chain) {
  return chain.blockExplorers?.default.url
}

if (typeof process.env.DEPLOYER_PRIVATE_KEY === 'undefined') {
  throw new Error(
    `Please provide the "DEPLOYER_PRIVATE_KEY" environment variable`
  )
}

if (typeof process.env.CUSTOM_FEE_TOKEN_ADDRESS === 'undefined') {
  throw new Error(
    `Please provide the "CUSTOM_FEE_TOKEN_ADDRESS" environment variable`
  )
}

if (
  typeof process.env.PARENT_CHAIN_RPC === 'undefined' ||
  process.env.PARENT_CHAIN_RPC === ''
) {
  console.warn(
    `Warning: you may encounter timeout errors while running the script with the default rpc endpoint. Please provide the "PARENT_CHAIN_RPC" environment variable instead.`
  )
}

// load or generate a random batch poster account
const batchPosterPrivateKey = withFallbackPrivateKey(
  process.env.BATCH_POSTER_PRIVATE_KEY
)
const batchPoster = privateKeyToAccount(batchPosterPrivateKey).address

// load or generate a random validator account
const validatorPrivateKey = withFallbackPrivateKey(
  process.env.VALIDATOR_PRIVATE_KEY
)
const validator = privateKeyToAccount(validatorPrivateKey).address

// set the parent chain and create a public client for it
// const parentChain = arbitrumSepolia
export const parentChain = updatedBscTestnet

const parentChainPublicClient = createPublicClient({
  chain: parentChain,
  transport: http(process.env.PARENT_CHAIN_RPC),
})

// load the deployer account
const deployer = privateKeyToAccount(
  sanitizePrivateKey(process.env.DEPLOYER_PRIVATE_KEY)
)

async function main() {
  // console.log('Somewhere is calling this script')
  // return
  registerCustomParentChain(updatedBscTestnet)
  // console.log(updatedBscTestnet);
  // return;

  // generate a random chain id
  const chainId = generateChainId()
  // set the custom fee token
  const nativeToken: Address = process.env
    .CUSTOM_FEE_TOKEN_ADDRESS as `0x${string}`

  // create the chain config
  const chainConfig = prepareChainConfig({
    chainId,
    arbitrum: {
      InitialChainOwner: deployer.address,
      DataAvailabilityCommittee: true,
    },
  })

  // console.log(chainConfig);
  // console.log(
  //     {
  //         batchPoster,
  //         batchPosterPrivateKey,
  //         validator,
  //         validatorPrivateKey
  //     }
  // )
  // return;

  const allowanceParams = {
    nativeToken,
    account: deployer.address,
    publicClient: parentChainPublicClient,
  }

  if (!(await createRollupEnoughCustomFeeTokenAllowance(allowanceParams))) {
    const approvalTxRequest =
      await createRollupPrepareCustomFeeTokenApprovalTransactionRequest(
        allowanceParams
      )

    // sign and send the transaction
    const approvalTxHash = await parentChainPublicClient.sendRawTransaction({
      serializedTransaction: await deployer.signTransaction(approvalTxRequest),
    })

    // get the transaction receipt after waiting for the transaction to complete
    const approvalTxReceipt = createRollupPrepareTransactionReceipt(
      await parentChainPublicClient.waitForTransactionReceipt({
        hash: approvalTxHash,
      })
    )

    console.log(
      `Tokens approved in ${getBlockExplorerUrl(parentChain)}/tx/${
        approvalTxReceipt.transactionHash
      }`
    )
  }

  // return;

  // prepare the transaction for deploying the core contracts
  const txRequest = await createRollupPrepareTransactionRequest({
    params: {
      config: createRollupPrepareDeploymentParamsConfig(
        parentChainPublicClient,
        {
          chainId: BigInt(chainId),
          owner: deployer.address,
          chainConfig,
          confirmPeriodBlocks: 150n,
          sequencerInboxMaxTimeVariation: {
            delayBlocks: BigInt((60 * 60 * 24 * 4) / 12),
            futureBlocks: BigInt((60 * 60) / 12),
            delaySeconds: BigInt(60 * 60 * 24 * 4),
            futureSeconds: BigInt(60 * 60),
          },
        }
      ),
      batchPosters: [batchPoster],
      validators: [validator],
      nativeToken,
      maxDataSize: 117964n,
    },
    account: deployer.address,
    publicClient: parentChainPublicClient,
  })

  // sign and send the transaction
  const txHash = await parentChainPublicClient.sendRawTransaction({
    serializedTransaction: await deployer.signTransaction(txRequest),
  })

  // get the transaction receipt after waiting for the transaction to complete
  const txReceipt = createRollupPrepareTransactionReceipt(
    await parentChainPublicClient.waitForTransactionReceipt({ hash: txHash })
  )

  console.log(
    `Deployed in ${getBlockExplorerUrl(parentChain)}/tx/${
      txReceipt.transactionHash
    }`
  )
}

main()
```

`npx esrun scripts/createRollup.ts`

# Sau đó là prepare chain config để chạy node.

`prepareChainConfig.ts`
```
import { writeFile } from 'fs/promises'
import { Chain, createPublicClient, http } from 'viem'
import { arbitrumSepolia } from 'viem/chains'
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts'

import {
  ChainConfig,
  PrepareNodeConfigParams,
  createRollupPrepareTransaction,
  createRollupPrepareTransactionReceipt,
  prepareNodeConfig,
} from '@arbitrum/orbit-sdk'
import { getParentChainLayer } from '@arbitrum/orbit-sdk/utils'
import { config } from 'dotenv'
import { contracts } from '../nitro/contracts/build/types/@openzeppelin'
import { updatedBscTestnet } from './utils'
config()

function getRpcUrl(chain: Chain) {
  return chain.rpcUrls.default.http[0]
}

// if (typeof process.env.ORBIT_DEPLOYMENT_TRANSACTION_HASH === 'undefined') {
//   throw new Error(
//     `Please provide the "ORBIT_DEPLOYMENT_TRANSACTION_HASH" environment variable`
//   )
// }

if (typeof process.env.BATCH_POSTER_PRIVATE_KEY === 'undefined') {
  throw new Error(
    `Please provide the "BATCH_POSTER_PRIVATE_KEY" environment variable`
  )
}

if (typeof process.env.VALIDATOR_PRIVATE_KEY === 'undefined') {
  throw new Error(
    `Please provide the "VALIDATOR_PRIVATE_KEY" environment variable`
  )
}

if (
  typeof process.env.PARENT_CHAIN_RPC === 'undefined' ||
  process.env.PARENT_CHAIN_RPC === ''
) {
  console.warn(
    `Warning: you may encounter timeout errors while running the script with the default rpc endpoint. Please provide the "PARENT_CHAIN_RPC" environment variable instead.`
  )
}

// set the parent chain and create a public client for it
// const parentChain = arbitrumSepolia
const parentChain = updatedBscTestnet
const parentChainPublicClient = createPublicClient({
  chain: parentChain,
  transport: http(process.env.PARENT_CHAIN_RPC),
})

// if (
//   getParentChainLayer(parentChainPublicClient.chain.id) == 1 &&
//   typeof process.env.ETHEREUM_BEACON_RPC_URL === 'undefined'
// ) {
//   throw new Error(
//     `Please provide the "ETHEREUM_BEACON_RPC_URL" environment variable necessary for L2 Orbit chains`
//   )
// }

async function main() {
  // tx hash for the transaction to create rollup
  const txHash =
    '0x0d9ed9bf096f36ad78cb3400faa5e07a1dee95b9c9db2d641941c41b585d788c' as `0x${string}`

  // get the transaction
  const tx = createRollupPrepareTransaction(
    await parentChainPublicClient.getTransaction({ hash: txHash })
  )

  // get the transaction receipt
  const txReceipt = createRollupPrepareTransactionReceipt(
    await parentChainPublicClient.getTransactionReceipt({ hash: txHash })
  )

  // get the chain config from the transaction inputs
  const chainConfig: ChainConfig = JSON.parse(
    tx.getInputs()[0].config.chainConfig
  )
  // get the core contracts from the transaction receipt
  const coreContracts = txReceipt.getCoreContracts()

  // prepare the node config
  const nodeConfigParameters: PrepareNodeConfigParams = {
    chainName: 'Medoo_Testnet_BSC_v1',
    chainConfig,
    coreContracts,
    batchPosterPrivateKey: process.env
      .BATCH_POSTER_PRIVATE_KEY as `0x${string}`,
    validatorPrivateKey: process.env.VALIDATOR_PRIVATE_KEY as `0x${string}`,
    parentChainId: arbitrumSepolia.id,
    parentChainRpcUrl:
      'https://site1.moralis-nodes.com/bsc-testnet/d7e9356ba4ec4480ba858b1d4c172608',
  }

  // // For L2 Orbit chains settling to Ethereum mainnet or testnet
  // if (getParentChainLayer(parentChainPublicClient.chain.id) === 1) {
  //   nodeConfigParameters.parentChainBeaconRpcUrl =
  //     process.env.ETHEREUM_BEACON_RPC_URL
  // }

  const nodeConfig = prepareNodeConfig(nodeConfigParameters)

  await writeFile('node-config.json', JSON.stringify(nodeConfig, null, 2))
  console.log(`Node config written to "node-config.json"`)

  const orbitSetupConfig = {
    networkFeeReceiver: '0x2aE0ce198408f86A77f3f4B6Bc301de8003C7Ce8',
    infrastructureFeeCollector: '0x2aE0ce198408f86A77f3f4B6Bc301de8003C7Ce8',
    staker: '0xE46c8D922fb21066D6Fb717492331bDDaA4E4c24',
    batchPoster: '0x87A1BD2CFEb539EC26EA73D4c0650Ad5aF62D244',
    chainOwner: '0x2aE0ce198408f86A77f3f4B6Bc301de8003C7Ce8',
    chainId: 28266744269,
    chainName: 'Medoo_Testnet_BSC_v1',
    minL2BaseFee: 100000000,
    parentChainId: 97,
    'parent-chain-node-url':
      'https://site1.moralis-nodes.com/bsc-testnet/d7e9356ba4ec4480ba858b1d4c172608',
    utils: coreContracts.validatorUtils,
    ...coreContracts,
  }
  await writeFile(
    'orbit-setup-script-config.json',
    JSON.stringify(orbitSetupConfig, null, 2)
  )
  console.log(`Node config written to "orbit-setup-script-config.json"`)
}

main()
```

`npx esrun scripts/prepareChainConfig.ts`

sửa lại file được build ra 1 xíu.

# Copy vào và chạy, chỉnh sửa các file env để thay đổi db name

copy nodeConfig.json và orbitSetupscript.json

docker-compose/envs/common-blockscout.env => DATABASE_URL // blockscout_testnet_bsc_v1
docker-compose/envs/common-frontend.env => NEXT_PUBLIC_NETWORK_ID
docker-compose/services/db.yml => db:POSTGRES_DB // stats_testnet_bsc_v1
docker-compose/services/stats.yml => stats-db:POSTGRES_DB, stats: STATS**DB_URL&STATS**BLOCKSCOUT_DB_URL

# Docker conmpose up -d và Chạy câu lệnh set up

trước khi chạy câu lệnh thì phải sửa 1 số phần trong script setup

`PRIVATE_KEY="0x980fd43ba377aa738a167cbd46b8b2c8c3ba5bef621a07c4afe57a35a59da781" L2_RPC_URL="https://site1.moralis-nodes.com/bsc-testnet/d7e9356ba4ec4480ba858b1d4c172608" L3_RPC_URL="http://localhost:8449" yarn run setup`

// Có thể chạy từ xa với tham số URL và orbitSetupScript.json
`PRIVATE_KEY="0x980fd43ba377aa738a167cbd46b8b2c8c3ba5bef621a07c4afe57a35a59da781" L2_RPC_URL="https://site1.moralis-nodes.com/bsc-testnet/d7e9356ba4ec4480ba858b1d4c172608" L3_RPC_URL="http://54.254.131.144:8449" yarn run setup`

30.515853436324813362 BNB // Ban đầu
30.41417 => 0.102 BNB // Deploy RollupCreator => 0xCAD3ae0523811b9d2eb3B26E12427ccE9D671c9A
30.3512 => 0.1646 BNB // Deploy TokenBridgeCreator => 0xB0C6B849cdd80Ee5D925C0454F151B75FC23CC8E
=> 0.1756 BNB // Deploy Rollup => 0x0d9ed9bf096f36ad78cb3400faa5e07a1dee95b9c9db2d641941c41b585d788c // txHash
