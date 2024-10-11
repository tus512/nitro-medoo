// Copyright 2021-2022, Offchain Labs, Inc.
// For license information, see https://github.com/nitro/blob/master/LICENSE

package blsSignatures

import (
	"bytes"
	"github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
	"math/rand"
	"testing"
	"time"

	"github.com/offchainlabs/nitro/util/testhelpers"
)

func TestPublicKeyFromPrivateKey(t *testing.T) {
	// Hardcoded private key for testing
	privateKey := new(fr.Element).SetBytes([]byte{54, 16, 51, 77, 200, 74, 139, 205, 66, 197, 218, 43, 163, 239, 159, 127, 31, 250, 204, 181, 30, 57, 125, 217, 57, 198, 145, 143, 232, 224, 117, 185})
	publicKey, err := PublicKeyFromPrivateKey(privateKey)
	Require(t, err)
	publicKeyBytes := publicKey.key.RawBytes()
	// Compare to the public key generated by the Ethereum BLS library
	if !bytes.Equal(publicKeyBytes[:], []byte{5, 26, 203, 151, 1, 124, 221, 162, 29, 13, 15, 227, 78, 232, 125, 200, 232, 245, 251, 196, 153, 185, 66, 74, 49, 126, 168, 225, 101, 252, 124, 184, 52, 101, 97, 135, 1, 36, 242, 88, 206, 106, 47, 226, 161, 148, 35, 61, 8, 234, 6, 124, 238, 224, 58, 64, 92, 163, 210, 25, 221, 204, 20, 149, 121, 193, 175, 168, 157, 184, 16, 216, 30, 181, 114, 184, 201, 251, 46, 246, 7, 80, 87, 34, 101, 34, 123, 51, 58, 176, 132, 118, 190, 53, 158, 161, 19, 144, 72, 109, 52, 189, 109, 245, 80, 64, 229, 196, 99, 200, 215, 204, 77, 156, 60, 196, 6, 167, 27, 227, 96, 190, 228, 57, 53, 32, 128, 67, 192, 155, 233, 163, 171, 83, 86, 81, 93, 20, 221, 52, 75, 254, 66, 42, 17, 79, 254, 35, 80, 175, 30, 100, 210, 109, 164, 150, 197, 88, 104, 152, 160, 178, 69, 78, 56, 215, 38, 180, 215, 212, 202, 233, 219, 224, 245, 184, 223, 248, 166, 91, 147, 62, 53, 61, 251, 83, 155, 92, 68, 201, 65, 92}) {
		Fail(t, "public key is incorrect")
	}
	// Use the validity proof generated by the Ethereum BLS library
	_, err = publicKey.validityProof.SetBytes([]byte{24, 152, 185, 33, 240, 229, 254, 108, 130, 235, 47, 25, 45, 224, 93, 56, 103, 226, 157, 91, 233, 2, 73, 218, 179, 213, 171, 7, 54, 4, 113, 43, 19, 25, 188, 71, 45, 232, 233, 95, 223, 113, 104, 118, 210, 115, 248, 126, 18, 80, 5, 160, 54, 207, 82, 154, 150, 84, 98, 19, 68, 17, 230, 124, 32, 106, 80, 143, 74, 214, 105, 109, 69, 114, 47, 239, 145, 131, 19, 145, 77, 207, 249, 122, 229, 239, 228, 89, 42, 207, 97, 244, 39, 21, 115, 60})
	Require(t, err)
	message := []byte("The quick brown fox jumped over the lazy dog.")
	sig, err := SignMessage(privateKey, message)
	Require(t, err)

	verified, err := VerifySignature(sig, message, publicKey)
	Require(t, err)
	if !verified {
		Fail(t, "valid signature failed to verify")
	}
}

func TestPublicKeyToBytes(t *testing.T) {
	expectedPublicKeyBytes := []byte{0, 5, 26, 203, 151, 1, 124, 221, 162, 29, 13, 15, 227, 78, 232, 125, 200, 232, 245, 251, 196, 153, 185, 66, 74, 49, 126, 168, 225, 101, 252, 124, 184, 52, 101, 97, 135, 1, 36, 242, 88, 206, 106, 47, 226, 161, 148, 35, 61, 8, 234, 6, 124, 238, 224, 58, 64, 92, 163, 210, 25, 221, 204, 20, 149, 121, 193, 175, 168, 157, 184, 16, 216, 30, 181, 114, 184, 201, 251, 46, 246, 7, 80, 87, 34, 101, 34, 123, 51, 58, 176, 132, 118, 190, 53, 158, 161, 19, 144, 72, 109, 52, 189, 109, 245, 80, 64, 229, 196, 99, 200, 215, 204, 77, 156, 60, 196, 6, 167, 27, 227, 96, 190, 228, 57, 53, 32, 128, 67, 192, 155, 233, 163, 171, 83, 86, 81, 93, 20, 221, 52, 75, 254, 66, 42, 17, 79, 254, 35, 80, 175, 30, 100, 210, 109, 164, 150, 197, 88, 104, 152, 160, 178, 69, 78, 56, 215, 38, 180, 215, 212, 202, 233, 219, 224, 245, 184, 223, 248, 166, 91, 147, 62, 53, 61, 251, 83, 155, 92, 68, 201, 65, 92}
	publicKey, err := PublicKeyFromBytes(expectedPublicKeyBytes, true)
	Require(t, err)
	publicKeyBytes := PublicKeyToBytes(publicKey)
	if !bytes.Equal(publicKeyBytes, expectedPublicKeyBytes) {
		Fail(t, "public key to bytes failed")
	}
}
func TestValidSignature(t *testing.T) {
	pub, priv, err := GenerateKeys()
	Require(t, err)

	message := []byte("The quick brown fox jumped over the lazy dog.")
	sig, err := SignMessage(priv, message)
	Require(t, err)

	verified, err := VerifySignature(sig, message, pub)
	Require(t, err)
	if !verified {
		Fail(t, "valid signature failed to verify")
	}
}

func TestWrongMessageSignature(t *testing.T) {
	pub, priv, err := GenerateKeys()
	Require(t, err)

	message := []byte("The quick brown fox jumped over the lazy dog.")
	sig, err := SignMessage(priv, message)
	Require(t, err)

	verified, err := VerifySignature(sig, append(message, 3), pub)
	Require(t, err)
	if verified {
		Fail(t, "signature check on wrong message didn't fail")
	}
}

func TestWrongKeySignature(t *testing.T) {
	_, priv, err := GenerateKeys()
	Require(t, err)
	pub, _, err := GenerateKeys()
	Require(t, err)

	message := []byte("The quick brown fox jumped over the lazy dog.")
	sig, err := SignMessage(priv, message)
	Require(t, err)

	verified, err := VerifySignature(sig, message, pub)
	Require(t, err)
	if verified {
		Fail(t, "signature check with wrong public key didn't fail")
	}
}

const NumSignaturesToAggregate = 12

func TestSignatureAggregation(t *testing.T) {
	message := []byte("The quick brown fox jumped over the lazy dog.")
	pubKeys := []PublicKey{}
	sigs := []Signature{}
	for i := 0; i < NumSignaturesToAggregate; i++ {
		pub, priv, err := GenerateKeys()
		Require(t, err)
		pubKeys = append(pubKeys, pub)
		sig, err := SignMessage(priv, message)
		Require(t, err)
		sigs = append(sigs, sig)
	}

	verified, err := VerifySignature(AggregateSignatures(sigs), message, AggregatePublicKeys(pubKeys))
	Require(t, err)
	if !verified {
		Fail(t, "First aggregated signature check failed")
	}

	verified, err = VerifyAggregatedSignatureSameMessage(AggregateSignatures(sigs), message, pubKeys)
	Require(t, err)
	if !verified {
		Fail(t, "Second aggregated signature check failed")
	}
}

func TestSignatureAggregationAnyOrder(t *testing.T) {
	message := []byte("The quick brown fox jumped over the lazy dog.")
	pubKeys := []PublicKey{}
	sigs := []Signature{}
	for i := 0; i < NumSignaturesToAggregate; i++ {
		pub, priv, err := GenerateKeys()
		Require(t, err)
		pubKeys = append(pubKeys, pub)
		sig, err := SignMessage(priv, message)
		Require(t, err)
		sigs = append(sigs, sig)
	}

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(NumSignaturesToAggregate, func(i, j int) { sigs[i], sigs[j] = sigs[j], sigs[i] })
	rand.Shuffle(NumSignaturesToAggregate, func(i, j int) { pubKeys[i], pubKeys[j] = pubKeys[j], pubKeys[i] })

	verified, err := VerifySignature(AggregateSignatures(sigs), message, AggregatePublicKeys(pubKeys))
	Require(t, err)
	if !verified {
		Fail(t, "First aggregated signature check failed")
	}

	rand.Shuffle(NumSignaturesToAggregate, func(i, j int) { sigs[i], sigs[j] = sigs[j], sigs[i] })
	verified, err = VerifyAggregatedSignatureSameMessage(AggregateSignatures(sigs), message, pubKeys)
	Require(t, err)
	if !verified {
		Fail(t, "Second aggregated signature check failed")
	}
}

func TestSignatureAggregationDifferentMessages(t *testing.T) {
	messages := [][]byte{}
	pubKeys := []PublicKey{}
	sigs := []Signature{}

	for i := 0; i < NumSignaturesToAggregate; i++ {
		msg := []byte{byte(i)}
		pubKey, privKey, err := GenerateKeys()
		Require(t, err)
		sig, err := SignMessage(privKey, msg)
		Require(t, err)
		messages = append(messages, msg)
		pubKeys = append(pubKeys, pubKey)
		sigs = append(sigs, sig)
	}

	verified, err := VerifyAggregatedSignatureDifferentMessages(AggregateSignatures(sigs), messages, pubKeys)
	Require(t, err)
	if !verified {
		Fail(t, "First aggregated signature check failed")
	}
}

func Require(t *testing.T, err error, printables ...interface{}) {
	t.Helper()
	testhelpers.RequireImpl(t, err, printables...)
}

func Fail(t *testing.T, printables ...interface{}) {
	t.Helper()
	testhelpers.FailImpl(t, printables...)
}
