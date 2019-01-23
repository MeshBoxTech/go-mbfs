package examples

import (
	"time"

	pb "mbfs/go-mbfs/gx/QmZMJfrt7fU33oFQ9WvWnovhiiZ8T6qkWkFXNCFreJTzgT/go-ipns/pb"

	crypto "mbfs/go-mbfs/gx/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	ipns "mbfs/go-mbfs/gx/QmZMJfrt7fU33oFQ9WvWnovhiiZ8T6qkWkFXNCFreJTzgT/go-ipns"
)

// CreateEntryWithEmbed shows how you can create an IPNS entry
// and embed it with a public key. For ed25519 keys this is not needed
// so attempting to embed with an ed25519 key, will not actually embed the key
func CreateEntryWithEmbed(ipfsPath string, publicKey crypto.PubKey, privateKey crypto.PrivKey) (*pb.IpnsEntry, error) {
	ipfsPathByte := []byte(ipfsPath)
	eol := time.Now().Add(time.Hour * 48)
	entry, err := ipns.Create(privateKey, ipfsPathByte, 1, eol)
	if err != nil {
		return nil, err
	}
	err = ipns.EmbedPublicKey(publicKey, entry)
	if err != nil {
		return nil, err
	}
	return entry, nil
}
