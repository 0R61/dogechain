package protocol

import (
	"context"
	"errors"
	"fmt"

	"github.com/dogechain-lab/dogechain/protocol/proto"
	"github.com/dogechain-lab/dogechain/types"
)

var (
	ErrNilHeaderResponse = errors.New("header response is nil")
)

func getHeaders(clt proto.V1Client, req *proto.GetHeadersRequest) ([]*types.Header, error) {
	resp, err := clt.GetHeaders(context.Background(), req)
	if err != nil {
		return nil, err
	}

	headers := []*types.Header{}

	for _, obj := range resp.Objs {
		if obj == nil || obj.Spec == nil {
			// this nil header comes from a faulty node, reject all blocks of it.
			return nil, ErrNilHeaderResponse
		}

		header := &types.Header{}
		if err := header.UnmarshalRLP(obj.Spec.Value); err != nil {
			return nil, err
		}

		headers = append(headers, header)
	}

	return headers, nil
}

type skeleton struct {
	slots []*slot
	span  int64
	num   int64
}

func (s *skeleton) LastHeader() *types.Header {
	slot := s.slots[len(s.slots)-1]

	return slot.blocks[len(slot.blocks)-1].Header
}

func (s *skeleton) build(clt proto.V1Client, ancestor types.Hash) error {
	// since ancestor is the common block we need to query the next one
	headers, err := getHeaders(clt, &proto.GetHeadersRequest{Hash: ancestor.String(), Skip: s.span - 1, Amount: s.num})
	if err != nil {
		return err
	}

	err = s.addSkeleton(headers)
	if err != nil {
		return err
	}

	return nil
}

func (s *skeleton) fillSlot(indx uint64, clt proto.V1Client) error {
	slot := s.slots[indx]
	req := &proto.GetHeadersRequest{
		Hash:   slot.hash.String(),
		Amount: s.span,
	}
	resp, err := getHeaders(clt, req)

	if err != nil {
		return err
	}

	// alloc the slice capacity once and for all
	slot.blocks = make([]*types.Block, 0, len(resp))

	for _, h := range resp {
		slot.blocks = append(slot.blocks, &types.Block{
			Header: h,
		})
	}

	// for each header with body we request it
	// alloc the slice capacity  once and for al
	bodyHashes := make([]types.Hash, 0, len(resp))
	bodyIndex := make([]int, 0, len(resp))

	for indx, h := range resp {
		if h.TxRoot != types.EmptyRootHash {
			bodyHashes = append(bodyHashes, h.Hash)
			bodyIndex = append(bodyIndex, indx)
		}
	}

	if len(bodyHashes) == 0 {
		return nil
	}

	bodies, err := getBodies(context.Background(), clt, bodyHashes)
	if err != nil {
		return err
	}

	for indx, body := range bodies {
		slot.blocks[bodyIndex[indx]].Transactions = body.Transactions
	}

	return nil
}

func (s *skeleton) addSkeleton(headers []*types.Header) error {
	// safe check make sure they all have the same difference
	diff := uint64(0)

	for i := 1; i < len(headers); i++ {
		elemDiff := headers[i].Number - headers[i-1].Number
		if diff == 0 {
			diff = elemDiff
		} else if elemDiff != diff {
			return fmt.Errorf("bad diff")
		}
	}

	// fill up the slots
	s.slots = make([]*slot, len(headers))

	for indx, header := range headers {
		slot := &slot{
			hash:   header.Hash,
			number: header.Number,
		}
		s.slots[indx] = slot
	}

	return nil
}

type slot struct {
	hash   types.Hash
	number uint64
	blocks []*types.Block
}
