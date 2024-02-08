package sequencer

import (
	"context"
	"crypto/ed25519"
	"fmt"

	astriaPb "buf.build/gen/go/astria/astria/protocolbuffers/go/astria/sequencer/v1alpha1"
	"github.com/astriaorg/go-sequencer-client/client"
	tendermintPb "github.com/cometbft/cometbft/rpc/core/types"
	"google.golang.org/protobuf/encoding/protojson"
)

// SequencerClient is a client for interacting with the sequencer.
type Client struct {
	Client  *client.Client
	Signer  *client.Signer
	chainId string
	nonce   uint32
}

func NewClient(sequencerAddr string, private ed25519.PrivateKey, chainId string) *Client {
	c, err := client.NewClient(sequencerAddr)
	if err != nil {
		panic(err)
	}

	return &Client{
		Client:  c,
		Signer:  client.NewSigner(private),
		chainId: chainId,
	}
}

func (c *Client) BroadcastTx(tx []byte) (*tendermintPb.ResultBroadcastTx, error) {
	unsigned := &astriaPb.UnsignedTransaction{
		Nonce: c.nonce,
		Actions: []*astriaPb.Action{
			{
				Value: &astriaPb.Action_SequenceAction{
					SequenceAction: &astriaPb.SequenceAction{
						RollupId: []byte(c.chainId),
						Data:     tx,
					},
				},
			},
		},
	}

	signed, err := c.Signer.SignTransaction(unsigned)
	if err != nil {
		return nil, err
	}

	signedJson, _ := protojson.Marshal(signed)
	fmt.Printf("submitting tx to sequencer: %s\n", signedJson)

	resp, err := c.Client.BroadcastTxSync(context.Background(), signed)
	if err != nil {
		return nil, err
	}

	if resp.Code == 4 {
		// fetch new nonce
		newNonce, err := c.Client.GetNonce(context.Background(), c.Signer.Address())
		if err != nil {
			return nil, err
		}
		c.nonce = newNonce

		// create new tx
		unsigned = &astriaPb.UnsignedTransaction{
			Nonce:   c.nonce,
			Actions: unsigned.Actions,
		}
		signed, err = c.Signer.SignTransaction(unsigned)
		if err != nil {
			return nil, err
		}

		// submit new tx
		resp, err = c.Client.BroadcastTxSync(context.Background(), signed)
		if err != nil {
			return nil, err
		}
		if resp.Code != 0 {
			return nil, fmt.Errorf("unexpected error code: %d", resp.Code)
		}
	} else if resp.Code != 0 {
		return nil, fmt.Errorf("unexpected error code: %d", resp.Code)
	}

	return resp, nil
}
