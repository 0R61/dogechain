package ibft

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dogechain-lab/dogechain/blockchain"
	"github.com/dogechain-lab/dogechain/consensus"
	"github.com/dogechain-lab/dogechain/consensus/ibft/proto"
	"github.com/dogechain-lab/dogechain/helper/common"
	"github.com/dogechain-lab/dogechain/helper/hex"
	"github.com/dogechain-lab/dogechain/helper/progress"
	"github.com/dogechain-lab/dogechain/protocol"
	"github.com/dogechain-lab/dogechain/state"
	"github.com/dogechain-lab/dogechain/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/atomic"
	anypb "google.golang.org/protobuf/types/known/anypb"
)

var (
	defaultBlockGasLimit uint64 = 8000000
)

type MockBlockchain struct {
	t *testing.T

	latestBlockNumber *uint64
	headers           map[uint64]*types.Header
	blocks            map[uint64]*types.Block

	// Handlers to change mock's behavior
	HeaderHandler               func() *types.Header
	GetHeaderByNumberHandler    func(uint64) (*types.Header, bool)
	WriteBlockHandler           func(*types.Block) error
	VerifyPotentialBlockHandler func(block *types.Block) error
	CalculateGasLimitHandler    func(number uint64) (uint64, error)
}

func (m *MockBlockchain) Header() *types.Header {
	m.t.Helper()

	if m.HeaderHandler == nil {
		m.errorByUndefinedMethod("Header")
	}

	return m.HeaderHandler()
}

func (m *MockBlockchain) GetHeaderByNumber(i uint64) (*types.Header, bool) {
	m.t.Helper()

	if m.GetHeaderByNumberHandler == nil {
		m.errorByUndefinedMethod("GetHeaderByNumber")
	}

	return m.GetHeaderByNumberHandler(i)
}

func (m *MockBlockchain) WriteBlock(block *types.Block) error {
	m.t.Helper()

	if m.WriteBlockHandler == nil {
		m.errorByUndefinedMethod("WriteBlock")
	}

	return m.WriteBlockHandler(block)
}

func (m *MockBlockchain) VerifyPotentialBlock(block *types.Block) error {
	m.t.Helper()

	if m.VerifyPotentialBlockHandler == nil {
		m.errorByUndefinedMethod("VerifyPotentialBlock")
	}

	return m.VerifyPotentialBlockHandler(block)
}

func (m *MockBlockchain) CalculateGasLimit(number uint64) (uint64, error) {
	m.t.Helper()

	if m.CalculateGasLimitHandler == nil {
		m.errorByUndefinedMethod("CalculateGasLimit")
	}

	return m.CalculateGasLimitHandler(number)
}

// helper method
func (m *MockBlockchain) SetGenesis(validators []types.Address) *types.Block {
	m.t.Helper()

	header := &types.Header{
		Number:     0,
		Difficulty: 0,
		ParentHash: types.ZeroHash,
		MixHash:    IstanbulDigest,
		Sha3Uncles: types.EmptyUncleHash,
		GasLimit:   defaultBlockGasLimit,
	}

	putIbftExtraValidators(header, validators)

	header = header.ComputeHash()

	block := &types.Block{
		Header: header,
	}

	if err := m.writeBlock(block); err != nil {
		m.t.Errorf("failed to insert genesis block: %v", err)
	}

	return block
}

func (m *MockBlockchain) MockBlock(
	height uint64,
	parentHash types.Hash,
	proposer *ecdsa.PrivateKey,
	validators []types.Address,
) *types.Block {
	m.t.Helper()

	var err error

	gasLimit, _ := m.CalculateGasLimit(height - 1)

	header := &types.Header{
		Number:     height,
		Difficulty: height,
		ParentHash: parentHash,
		MixHash:    IstanbulDigest,
		Sha3Uncles: types.EmptyUncleHash,
		GasLimit:   gasLimit,
	}

	putIbftExtraValidators(header, validators)

	header = header.ComputeHash()

	header, err = writeSeal(proposer, header)
	if err != nil {
		m.t.Errorf("failed to write seal in DummyBlock: %v", err)
	}

	return &types.Block{
		Header: header,
	}
}

func (m *MockBlockchain) errorByUndefinedMethod(methodName string) {
	m.t.Helper()
	m.t.Errorf("%s method is not defined in MockBlockchain", methodName)
}

// default behaviors
func (m *MockBlockchain) header() *types.Header {
	if m.latestBlockNumber == nil {
		return nil
	}

	return m.headers[*m.latestBlockNumber]
}

func (m *MockBlockchain) getHeaderByNumber(i uint64) (*types.Header, bool) {
	header, ok := m.headers[i]

	return header, ok
}

func (m *MockBlockchain) writeBlock(block *types.Block) error {
	number := block.Number()
	m.blocks[number] = block

	if _, ok := m.headers[number]; !ok {
		m.headers[number] = block.Header

		if m.latestBlockNumber == nil || *m.latestBlockNumber < number {
			m.latestBlockNumber = &number
		}
	}

	return nil
}

func (m *MockBlockchain) verifyPotentialBlock(block *types.Block) error {
	return nil
}

func (m *MockBlockchain) calculateGasLimit(number uint64) (uint64, error) {
	return defaultBlockGasLimit, nil
}

// interface check
var _ blockchainInterface = (*MockBlockchain)(nil)

func NewMockBlockchain(t *testing.T) *MockBlockchain {
	t.Helper()

	m := &MockBlockchain{
		t:                 t,
		latestBlockNumber: nil,
		headers:           make(map[uint64]*types.Header),
		blocks:            make(map[uint64]*types.Block),
	}

	m.HeaderHandler = m.header
	m.GetHeaderByNumberHandler = m.getHeaderByNumber
	m.WriteBlockHandler = m.writeBlock
	m.VerifyPotentialBlockHandler = m.verifyPotentialBlock
	m.CalculateGasLimitHandler = m.calculateGasLimit

	return m
}

func TestTransition_ValidateState_Prepare(t *testing.T) {
	t.Skip()

	// we receive enough prepare messages to lock and commit the block
	i := newMockIbft(t, []string{"A", "B", "C", "D"}, "A")
	i.setState(ValidateState)

	i.emitMsg(&proto.MessageReq{
		From: "A",
		Type: proto.MessageReq_Prepare,
		View: proto.ViewMsg(1, 0),
	})
	i.emitMsg(&proto.MessageReq{
		From: "B",
		Type: proto.MessageReq_Prepare,
		View: proto.ViewMsg(1, 0),
	})
	// repeated message is not included
	i.emitMsg(&proto.MessageReq{
		From: "B",
		Type: proto.MessageReq_Prepare,
		View: proto.ViewMsg(1, 0),
	})
	i.emitMsg(&proto.MessageReq{
		From: "C",
		Type: proto.MessageReq_Prepare,
		View: proto.ViewMsg(1, 0),
	})
	i.Close()

	i.runCycle()

	i.expect(expectResult{
		sequence:    1,
		state:       ValidateState,
		prepareMsgs: 3,
		commitMsgs:  1, // A commit message
		locked:      true,
		outgoing:    1, // A commit message
	})
}

func TestTransition_ValidateState_CommitFastTrack(t *testing.T) {
	t.Skip()

	// we can directly receive the commit messages and fast track to the commit state
	// even when we do not have yet the preprepare messages
	i := newMockIbft(t, []string{"A", "B", "C", "D"}, "A")

	seal := hex.EncodeToHex(make([]byte, IstanbulExtraSeal))

	i.setState(ValidateState)
	i.state.view = proto.ViewMsg(1, 0)
	i.state.block = i.DummyBlock()
	i.state.locked = true

	i.emitMsg(&proto.MessageReq{
		From: "A",
		Type: proto.MessageReq_Commit,
		View: proto.ViewMsg(1, 0),
		Seal: seal,
	})
	i.emitMsg(&proto.MessageReq{
		From: "B",
		Type: proto.MessageReq_Commit,
		View: proto.ViewMsg(1, 0),
		Seal: seal,
	})
	i.emitMsg(&proto.MessageReq{
		From: "B",
		Type: proto.MessageReq_Commit,
		View: proto.ViewMsg(1, 0),
		Seal: seal,
	})
	i.emitMsg(&proto.MessageReq{
		From: "C",
		Type: proto.MessageReq_Commit,
		View: proto.ViewMsg(1, 0),
		Seal: seal,
	})

	i.runCycle()

	i.expect(expectResult{
		sequence:   1,
		commitMsgs: 3,
		outgoing:   1,
		locked:     false, // unlock after commit
	})
}

func TestTransition_AcceptState_ToSync(t *testing.T) {
	// we are in AcceptState and we are not in the validators list
	// means that we have been removed as validator, move to sync state
	i := newMockIbft(t, []string{"A", "B", "C", "D"}, "")
	i.setState(AcceptState)
	i.Close()

	// we are the proposer and we need to build a block
	i.runCycle()

	i.expect(expectResult{
		sequence: 1,
		state:    SyncState,
	})
}

func TestTransition_AcceptState_Proposer_Locked(t *testing.T) {
	// If we are the proposer and there is a lock value we need to propose it
	i := newMockIbft(t, []string{"A", "B", "C", "D"}, "A")
	i.setState(AcceptState)

	i.state.locked = true
	i.state.block = &types.Block{
		Header: &types.Header{
			Number: 10,
		},
	}

	i.runCycle()

	i.expect(expectResult{
		sequence: 1,
		state:    ValidateState,
		locked:   true,
		outgoing: 2, // preprepare and prepare
	})

	if i.state.block.Number() != 10 {
		t.Fatal("bad block")
	}
}

func TestTransition_AcceptState_Validator_VerifyCorrect(t *testing.T) {
	i := newMockIbft(t, []string{"A", "B", "C"}, "B")
	i.state.view = proto.ViewMsg(1, 0)
	i.setState(AcceptState)

	block := i.DummyBlock()
	header, err := writeSeal(i.pool.get("A").priv, block.Header)

	assert.NoError(t, err)

	block.Header = header

	// A sends the message
	i.emitMsg(&proto.MessageReq{
		From: "A",
		Type: proto.MessageReq_Preprepare,
		Proposal: &anypb.Any{
			Value: block.MarshalRLP(),
		},
		View: proto.ViewMsg(1, 0),
	})

	i.runCycle()

	i.expect(expectResult{
		sequence: 1,
		state:    ValidateState,
		outgoing: 1, // prepare
	})
}

func TestTransition_AcceptState_Validator_VerifyFails(t *testing.T) {
	i := newMockIbft(t, []string{"A", "B", "C"}, "B")
	i.state.view = proto.ViewMsg(1, 0)
	i.setState(AcceptState)

	block := i.DummyBlock()
	block.Header.MixHash = types.Hash{} // invalidates the block

	header, err := writeSeal(i.pool.get("A").priv, block.Header)

	assert.NoError(t, err)

	block.Header = header

	// A sends the message
	i.emitMsg(&proto.MessageReq{
		From: "A",
		Type: proto.MessageReq_Preprepare,
		Proposal: &anypb.Any{
			Value: block.MarshalRLP(),
		},
		View: proto.ViewMsg(1, 0),
	})

	i.runCycle()

	i.expect(expectResult{
		sequence: 1,
		state:    RoundChangeState,
		err:      errBlockVerificationFailed,
	})
}

func TestTransition_AcceptState_Validator_ProposerInvalid(t *testing.T) {
	i := newMockIbft(t, []string{"A", "B", "C"}, "B")
	i.state.view = proto.ViewMsg(1, 0)
	i.setState(AcceptState)

	// A is the proposer but C sends the propose, we do not fail
	// but wait for timeout to move to roundChange state
	i.emitMsg(&proto.MessageReq{
		From: "C",
		Type: proto.MessageReq_Preprepare,
		Proposal: &anypb.Any{
			Value: i.DummyBlock().MarshalRLP(),
		},
		View: proto.ViewMsg(1, 0),
	})
	i.forceTimeout()

	i.runCycle()

	i.expect(expectResult{
		sequence: 1,
		state:    RoundChangeState,
	})
}

func TestTransition_AcceptState_Validator_LockWrong(t *testing.T) {
	i := newMockIbft(t, []string{"A", "B", "C"}, "B")
	i.state.view = proto.ViewMsg(1, 0)
	i.setState(AcceptState)

	// locked block
	block := i.DummyBlock()
	block.Header.Number = 1
	block.Header.ComputeHash()

	i.state.block = block
	i.state.locked = true

	// proposed block
	block1 := i.DummyBlock()
	block1.Header.Number = 2
	block1.Header.ComputeHash()

	i.emitMsg(&proto.MessageReq{
		From: "A",
		Type: proto.MessageReq_Preprepare,
		Proposal: &anypb.Any{
			Value: block1.MarshalRLP(),
		},
		View: proto.ViewMsg(1, 0),
	})

	i.runCycle()

	i.expect(expectResult{
		sequence: 1,
		state:    RoundChangeState,
		locked:   true,
		err:      errIncorrectBlockHeight,
	})
}

func TestTransition_AcceptState_Validator_LockCorrect(t *testing.T) {
	i := newMockIbft(t, []string{"A", "B", "C"}, "B")
	i.state.view = proto.ViewMsg(1, 0)
	i.setState(AcceptState)

	// locked block
	block := i.DummyBlock()
	block.Header.Number = 1
	block.Header.ComputeHash()

	i.state.block = block
	i.state.locked = true

	i.emitMsg(&proto.MessageReq{
		From: "A",
		Type: proto.MessageReq_Preprepare,
		Proposal: &anypb.Any{
			Value: block.MarshalRLP(),
		},
		View: proto.ViewMsg(1, 0),
	})

	i.runCycle()

	i.expect(expectResult{
		sequence: 1,
		state:    ValidateState,
		locked:   true,
		outgoing: 1, // prepare message
	})
}

// Test whether a validator rejects the proposed block with the wrong height
func TestTransition_AcceptState_Reject_WrongHeight_Block(t *testing.T) {
	pool := newTesterAccountPool()
	pool.add("A", "B", "C", "D")

	blockchain := NewMockBlockchain(t)
	blockchain.SetGenesis(pool.ValidatorSet())

	i := newMockIBFTWithMockBlockchain(t, pool, blockchain, "A")

	// Initialize IBFT state
	var (
		// The next sequence validator enters
		nextSequence uint64 = 2

		// The height in the next proposed block. This height doesn't match the sequence
		proposeBlockHeight uint64 = 3

		// The latest block in the chain
		latestBlock = blockchain.MockBlock(nextSequence-1, types.ZeroHash, pool.get("B").priv, pool.ValidatorSet())

		// The next proposed block in the network
		proposedBlock = blockchain.MockBlock(proposeBlockHeight, types.ZeroHash, pool.get("C").priv, pool.ValidatorSet())
	)

	i.state.view = proto.ViewMsg(nextSequence, 0)
	i.setState(AcceptState)

	blockchain.HeaderHandler = func() *types.Header {
		return latestBlock.Header
	}

	i.emitMsg(&proto.MessageReq{
		From: "C",
		Type: proto.MessageReq_Preprepare,
		Proposal: &anypb.Any{
			Value: proposedBlock.MarshalRLP(),
		},
		// Proposer propose block #3 but set sequence 2 in View in order to pass message queue in other validators
		View: proto.ViewMsg(nextSequence, 0),
	})

	i.runCycle()

	i.expect(expectResult{
		sequence: nextSequence,
		state:    RoundChangeState,
		locked:   false,
		outgoing: 0,
		err:      errIncorrectBlockHeight,
	})
}

func TestTransition_RoundChangeState_CatchupRound(t *testing.T) {
	m := newMockIbft(t, []string{"A", "B", "C", "D"}, "A")
	m.setState(RoundChangeState)

	// new messages arrive with round number 2
	m.emitMsg(&proto.MessageReq{
		From: "B",
		Type: proto.MessageReq_RoundChange,
		View: proto.ViewMsg(1, 2),
	})
	m.emitMsg(&proto.MessageReq{
		From: "C",
		Type: proto.MessageReq_RoundChange,
		View: proto.ViewMsg(1, 2),
	})
	m.emitMsg(&proto.MessageReq{
		From: "D",
		Type: proto.MessageReq_RoundChange,
		View: proto.ViewMsg(1, 2),
	})
	m.Close()

	// as soon as it starts it will move to round 1 because it has
	// not processed all the messages yet.
	// After it receives 3 Round change messages higher than his own
	// round it will change round again and move to accept
	m.runCycle()

	m.expect(expectResult{
		sequence: 1,
		round:    2,
		outgoing: 1, // our new round change
		state:    AcceptState,
	})
}

func TestTransition_RoundChangeState_Timeout(t *testing.T) {
	m := newMockIbft(t, []string{"A", "B", "C", "D"}, "A")

	m.forceTimeout()
	m.setState(RoundChangeState)
	m.Close()

	// increases to round 1 at the beginning of the round and sends
	// one RoundChange message.
	// After the timeout, it increases to round 2 and sends another
	/// RoundChange message.
	m.runCycle()

	m.expect(expectResult{
		sequence: 1,
		round:    2,
		outgoing: 2, // two round change messages
		state:    RoundChangeState,
	})
}

func TestTransition_RoundChangeState_WeakCertificate(t *testing.T) {
	m := newMockIbft(t, []string{"A", "B", "C", "D", "E", "F", "G"}, "A")
	m.setState(RoundChangeState)

	// send three roundChange messages which are enough to force a
	// weak change where the client moves to that new round state
	m.emitMsg(&proto.MessageReq{
		From: "B",
		Type: proto.MessageReq_RoundChange,
		View: proto.ViewMsg(1, 2),
	})
	m.emitMsg(&proto.MessageReq{
		From: "C",
		Type: proto.MessageReq_RoundChange,
		View: proto.ViewMsg(1, 2),
	})
	m.emitMsg(&proto.MessageReq{
		From: "D",
		Type: proto.MessageReq_RoundChange,
		View: proto.ViewMsg(1, 2),
	})
	m.Close()

	m.runCycle()

	m.expect(expectResult{
		sequence: 1,
		round:    2,
		outgoing: 2, // two round change messages (0->1, 1->2 after weak certificate)
		state:    RoundChangeState,
	})
}

func TestTransition_RoundChangeState_ErrStartNewRound(t *testing.T) {
	// if we start a round change because there was an error we start
	// a new round right away
	m := newMockIbft(t, []string{"A", "B"}, "A")
	m.Close()

	m.state.err = errBlockVerificationFailed

	m.setState(RoundChangeState)
	m.runCycle()

	m.expect(expectResult{
		sequence: 1,
		round:    1,
		state:    RoundChangeState,
		outgoing: 1,
	})
}

func TestTransition_RoundChangeState_StartNewRound(t *testing.T) {
	// if we start round change due to a state timeout and we are on the
	// correct sequence, we start a new round
	m := newMockIbft(t, []string{"A", "B"}, "A")
	m.Close()

	m.state.view.Sequence = 1

	m.setState(RoundChangeState)
	m.runCycle()

	m.expect(expectResult{
		sequence: 1,
		round:    1,
		state:    RoundChangeState,
		outgoing: 1,
	})
}

func TestTransition_RoundChangeState_MaxRound(t *testing.T) {
	// if we start round change due to a state timeout we try to catch up
	// with the highest round seen.
	m := newMockIbft(t, []string{"A", "B", "C"}, "A")
	m.Close()

	m.addMessage(&proto.MessageReq{
		From: "B",
		Type: proto.MessageReq_RoundChange,
		View: &proto.View{
			Round:    10,
			Sequence: 1,
		},
	})

	m.setState(RoundChangeState)
	m.runCycle()

	m.expect(expectResult{
		sequence: 1,
		round:    10,
		state:    RoundChangeState,
		outgoing: 1,
	})
}

func TestIBFT_WriteTransactions(t *testing.T) {
	type testParams struct {
		txns                        []*types.Transaction
		failedTxnsIndexes           []int
		notExecutableTxnsIndexes    int
		gasLimitReachedTxnIndex     int
		expectedIncludedTxnsCount   int
		expectedFailReceiptsWritten int
		expectedDropTxnsCount       int
		expectedDemoteTxnsCount     int
	}

	type testCase struct {
		description string
		params      testParams
	}

	setupMockTransition := func(test testCase, mockTxPool *mockTxPool) *mockTransition {
		mockTransition := &mockTransition{}
		for _, i := range test.params.failedTxnsIndexes {
			mockTransition.failReceiptsWritten = append(
				mockTransition.failReceiptsWritten,
				mockTxPool.transactions[i],
			)
		}

		if test.params.notExecutableTxnsIndexes >= 0 {
			mockTransition.shouldDroppedTransactions = append(
				mockTransition.shouldDroppedTransactions[:0],
				mockTxPool.transactions[test.params.notExecutableTxnsIndexes:]...,
			)
		}

		if test.params.gasLimitReachedTxnIndex > 0 {
			mockTransition.gasLimitReachedTransaction = mockTxPool.transactions[test.params.gasLimitReachedTxnIndex]
		}

		return mockTransition
	}

	testCases := []testCase{
		{
			"valid transaction is included in transition",
			testParams{
				txns: []*types.Transaction{
					{Nonce: 1},
				},
				notExecutableTxnsIndexes:    -1,
				gasLimitReachedTxnIndex:     -1,
				expectedIncludedTxnsCount:   1,
				expectedFailReceiptsWritten: 0,
				expectedDropTxnsCount:       0,
				expectedDemoteTxnsCount:     0,
			},
		},
		{
			"transaction whose gas exceeds block gas limit is included but with failedReceipt",
			testParams{
				txns: []*types.Transaction{
					{Nonce: 1},
					{Nonce: 2, Gas: 10001}, // exceeds block gas limit, included with failed receipt
					{Nonce: 3},
					{Nonce: 4},
				},
				notExecutableTxnsIndexes:    -1,
				gasLimitReachedTxnIndex:     1,
				expectedIncludedTxnsCount:   1, // nonce 1
				expectedFailReceiptsWritten: 1, // nonce 2
				expectedDropTxnsCount:       1, // nonce 2
				expectedDemoteTxnsCount:     0,
			},
		},
		{
			"transactions execute failed should be included too",
			testParams{
				txns: []*types.Transaction{
					{Nonce: 1},
					{Nonce: 2},
					{Nonce: 3}, // failed, should be included, too
					{Nonce: 4},
				},
				failedTxnsIndexes:           []int{2},
				notExecutableTxnsIndexes:    -1,
				gasLimitReachedTxnIndex:     -1,
				expectedIncludedTxnsCount:   4, // nonce 1, 2, 3, 4
				expectedFailReceiptsWritten: 1, // nonce 3
				expectedDropTxnsCount:       0,
				expectedDemoteTxnsCount:     0,
			},
		},
		{
			"transaction not executable account should be dropped",
			testParams{
				txns: []*types.Transaction{
					{Nonce: 1},
					{Nonce: 2}, // not executable, should be dropped, too
					{Nonce: 3},
					{Nonce: 4},
				},
				notExecutableTxnsIndexes:    1, // nonce 2
				gasLimitReachedTxnIndex:     -1,
				expectedIncludedTxnsCount:   1, // nonce 1
				expectedFailReceiptsWritten: 0,
				expectedDropTxnsCount:       1, // nonce 1
				expectedDemoteTxnsCount:     0,
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			m := newMockIbft(t, []string{"A", "B", "C"}, "A")
			mockTxPool := newMockTxPool(test.params.txns)
			m.txpool = mockTxPool
			mockTransition := setupMockTransition(test, mockTxPool)

			included, shouldDropTxs, shouldDemoteTxs := m.writeTransactions(1000, mockTransition)

			assert.Equal(t, test.params.expectedIncludedTxnsCount, len(included))
			assert.Equal(t, test.params.expectedFailReceiptsWritten, len(mockTransition.failReceiptsWritten))
			assert.Equal(t, test.params.expectedDropTxnsCount, len(shouldDropTxs))
			assert.Equal(t, test.params.expectedDemoteTxnsCount, len(shouldDemoteTxs))
		})
	}
}

func TestRunSyncState_NewHeadReceivedFromPeer_CallsTxPoolResetWithHeaders(t *testing.T) {
	m := newMockIbft(t, []string{"A", "B", "C"}, "A")
	m.setState(SyncState)

	expectedNewBlockToSync := &types.Block{Header: &types.Header{Number: 1}}
	mockSyncer := newMockSyncer(nil, expectedNewBlockToSync, nil, false, nil)
	m.syncer = mockSyncer
	mockTxPool := newMockTxPool(nil)
	m.txpool = mockTxPool

	// we need to change state from Sync in order to break from the loop inside runSyncState
	stateChangeDelay := time.NewTimer(100 * time.Millisecond)

	go func() {
		<-stateChangeDelay.C
		m.setState(AcceptState)
	}()

	m.runSyncState()

	assert.True(t, mockTxPool.resetWithHeaderCalled)
	assert.Equal(t, expectedNewBlockToSync.Header, mockTxPool.resetWithHeadersParam[0])
	assert.True(t, mockSyncer.broadcastCalled)
	assert.Equal(t, expectedNewBlockToSync, mockSyncer.broadcastedBlock)
}

func TestRunSyncState_BulkSyncWithPeer_CallsTxPoolResetWithHeaders(t *testing.T) {
	m := newMockIbft(t, []string{"A", "B", "C"}, "A")
	m.setState(SyncState)

	expectedNewBlocksToSync := []*types.Block{
		{Header: &types.Header{Number: 1}},
		{Header: &types.Header{Number: 2}},
		{Header: &types.Header{Number: 3}},
	}
	m.syncer = newMockSyncer(expectedNewBlocksToSync, nil, nil, false, nil)
	mockTxPool := newMockTxPool(nil)
	m.txpool = mockTxPool

	// we need to change state from Sync in order to break from the loop inside runSyncState
	stateChangeDelay := time.NewTimer(100 * time.Millisecond)

	go func() {
		<-stateChangeDelay.C
		m.setState(AcceptState)
	}()

	m.runSyncState()

	assert.True(t, mockTxPool.resetWithHeaderCalled)
	assert.Equal(t,
		expectedNewBlocksToSync[len(expectedNewBlocksToSync)-1].Header,
		mockTxPool.resetWithHeadersParam[0],
	)
}

// Tests whether validator unlock block if it syncs blocks during sync process
func TestRunSyncState_Unlock_After_Sync(t *testing.T) {
	pool := newTesterAccountPool()
	pool.add("A", "B", "C", "D")

	blockchain := NewMockBlockchain(t)
	blockchain.SetGenesis(pool.ValidatorSet())

	m := newMockIBFTWithMockBlockchain(t, pool, blockchain, "A")
	m.sealing = true
	m.setState(SyncState)

	// Locking block #1
	m.state.locked = true

	// Sync blocks to #3
	expectedNewBlocksToSync := []*types.Block{
		{Header: &types.Header{Number: 1}},
		{Header: &types.Header{Number: 2}},
		{Header: &types.Header{Number: 3}},
	}

	m.syncer = newMockSyncer(expectedNewBlocksToSync, nil, nil, false, blockchain)
	m.txpool = newMockTxPool(nil)

	// we need to change state from Sync in order to break from the loop inside runSyncState
	stateChangeDelay := time.NewTimer(100 * time.Millisecond)

	go func() {
		<-stateChangeDelay.C
		m.setState(AcceptState)
	}()

	m.runSyncState()

	// Validator should start new sequence from the next of the latest block and unlock block in state
	m.expect(expectResult{
		sequence: 4,
		state:    AcceptState,
		locked:   false,
	})
}

type mockSyncer struct {
	bulkSyncBlocksFromPeer  []*types.Block
	receivedNewHeadFromPeer *types.Block
	broadcastedBlock        *types.Block
	broadcastCalled         bool
	blockchain              blockchainInterface
}

func newMockSyncer(
	bulkSyncBlocksFromPeer []*types.Block,
	receivedNewHeadFromPeer *types.Block,
	broadcastedBlock *types.Block,
	broadcastCalled bool,
	blockchain blockchainInterface,
) *mockSyncer {
	return &mockSyncer{
		bulkSyncBlocksFromPeer:  bulkSyncBlocksFromPeer,
		receivedNewHeadFromPeer: receivedNewHeadFromPeer,
		broadcastedBlock:        broadcastedBlock,
		broadcastCalled:         broadcastCalled,
		blockchain:              blockchain,
	}
}

func (s *mockSyncer) Start() {}

func (s *mockSyncer) BestPeer() *protocol.SyncPeer {
	return &protocol.SyncPeer{}
}

func (s *mockSyncer) BulkSyncWithPeer(p *protocol.SyncPeer, handler func(block *types.Block)) error {
	for _, block := range s.bulkSyncBlocksFromPeer {
		if s.blockchain != nil {
			if err := s.blockchain.WriteBlock(block); err != nil {
				return err
			}
		}

		handler(block)
	}

	return nil
}

func (s *mockSyncer) WatchSyncWithPeer(
	p *protocol.SyncPeer,
	newBlockHandler func(b *types.Block) bool,
	blockTimeout time.Duration,
) {
	if s.receivedNewHeadFromPeer != nil {
		if s.blockchain != nil {
			if err := s.blockchain.WriteBlock(s.receivedNewHeadFromPeer); err != nil {
				return
			}
		}

		newBlockHandler(s.receivedNewHeadFromPeer)
	}
}

func (s *mockSyncer) GetSyncProgression() *progress.Progression {
	return nil
}

func (s *mockSyncer) Broadcast(b *types.Block) {
	s.broadcastCalled = true
	s.broadcastedBlock = b
}

type mockTxPool struct {
	transactions          []*types.Transaction
	demoted               []*types.Transaction
	nonceDecreased        map[*types.Transaction]bool
	resetWithHeaderCalled bool
	resetWithHeadersParam []*types.Header
}

func newMockTxPool(txs []*types.Transaction) *mockTxPool {
	return &mockTxPool{
		transactions: txs,
	}
}

// interface check
var _ txPoolInterface = (*mockTxPool)(nil)

func (p *mockTxPool) RemoveExecuted(tx *types.Transaction) {
	// do nothing
}

func (p *mockTxPool) DemoteAllPromoted(tx *types.Transaction, correctNonce uint64) {
	p.RemoveExecuted(tx)
	p.demoted = append(p.demoted, tx)
}

func (p *mockTxPool) Drop(tx *types.Transaction) {
	if p.nonceDecreased == nil {
		p.nonceDecreased = make(map[*types.Transaction]bool)
	}

	p.RemoveExecuted(tx)
	p.nonceDecreased[tx] = true
}

func (p *mockTxPool) ResetWithHeaders(headers ...*types.Header) {
	p.resetWithHeaderCalled = true
	p.resetWithHeadersParam = headers
}

func (p *mockTxPool) Pending() map[types.Address][]*types.Transaction {
	txs := make(map[types.Address][]*types.Transaction)

	for _, tx := range p.transactions {
		txs[tx.From] = append(txs[tx.From], tx)
	}

	return txs
}

type mockTransition struct {
	failReceiptsWritten        []*types.Transaction
	shouldDroppedTransactions  []*types.Transaction
	successReceiptsWritten     []*types.Transaction
	gasLimitReachedTransaction *types.Transaction
}

func (t *mockTransition) WriteFailedReceipt(txn *types.Transaction) error {
	t.failReceiptsWritten = append(t.failReceiptsWritten, txn)

	return nil
}

func (t *mockTransition) Write(txn *types.Transaction) error {
	if txn == t.gasLimitReachedTransaction {
		return state.NewGasLimitReachedTransitionApplicationError(nil)
	}

	for _, droppedTx := range t.shouldDroppedTransactions {
		if txn == droppedTx {
			return errors.New("mock not executable tx")
		}
	}

	t.successReceiptsWritten = append(t.successReceiptsWritten, txn)

	return nil
}

type mockIbft struct {
	t *testing.T
	*Ibft

	blockchain blockchainInterface
	pool       *testerAccountPool
	respMsg    []*proto.MessageReq
}

func (m *mockIbft) DummyBlock() *types.Block {
	parent, ok := m.blockchain.GetHeaderByNumber(0)
	assert.True(m.t, ok, "genesis block not found")

	num := parent.Number + 1
	gasLimit, err := m.CalculateGasLimit(num)
	assert.NoError(m.t, err, "failed to calculate next gas limit")

	block := &types.Block{
		Header: &types.Header{
			Number:     num,
			Difficulty: num,
			ParentHash: parent.Hash,
			ExtraData:  parent.ExtraData,
			MixHash:    IstanbulDigest,
			Sha3Uncles: types.EmptyUncleHash,
			GasLimit:   gasLimit,
		},
	}

	return block
}

func (m *mockIbft) Header() *types.Header {
	return m.blockchain.Header()
}

func (m *mockIbft) GetHeaderByNumber(i uint64) (*types.Header, bool) {
	return m.blockchain.GetHeaderByNumber(i)
}

func (m *mockIbft) WriteBlock(block *types.Block) error {
	return nil
}

func (m *mockIbft) VerifyPotentialBlock(block *types.Block) error {
	return nil
}

func (m *mockIbft) emitMsg(msg *proto.MessageReq) {
	// convert the address from the address pool
	from := m.pool.get(msg.From).Address()
	msg.From = from.String()

	m.Ibft.pushMessage(msg)
}

func (m *mockIbft) addMessage(msg *proto.MessageReq) {
	// convert the address from the address pool
	from := m.pool.get(msg.From).Address()
	msg.From = from.String()

	m.state.addMessage(msg)
}

func (m *mockIbft) Gossip(msg *proto.MessageReq) error {
	m.respMsg = append(m.respMsg, msg)

	return nil
}

func (m *mockIbft) CalculateGasLimit(number uint64) (uint64, error) {
	return m.blockchain.CalculateGasLimit(number)
}

func newMockIbft(t *testing.T, accounts []string, validatorAccount string) *mockIbft {
	t.Helper()

	pool := newTesterAccountPool()
	pool.add(accounts...)

	m := &mockIbft{
		t:          t,
		pool:       pool,
		blockchain: blockchain.TestBlockchain(t, pool.genesis()),
		respMsg:    []*proto.MessageReq{},
	}

	var addr *testerAccount

	if validatorAccount == "" {
		// account not in validator set, create a new one that is not part
		// of the genesis
		pool.add("xx")
		addr = pool.get("xx")
	} else {
		addr = pool.get(validatorAccount)
	}

	ibft := &Ibft{
		logger:           hclog.NewNullLogger(),
		config:           &consensus.Config{},
		blockchain:       m,
		validatorKey:     addr.priv,
		validatorKeyAddr: addr.Address(),
		closeCh:          make(chan struct{}),
		isClosed:         atomic.NewBool(false),
		updateCh:         make(chan struct{}),
		operator:         &operator{},
		state:            newState(),
		epochSize:        DefaultEpochSize,
		metrics:          consensus.NilMetrics(),
	}

	initIbftMechanism(PoA, ibft)

	// by default set the state to (1, 0)
	ibft.state.view = proto.ViewMsg(1, 0)

	m.Ibft = ibft

	assert.NoError(t, ibft.setupSnapshot())
	assert.NoError(t, ibft.createKey())

	// set the initial validators frrom the snapshot
	ibft.state.validators = pool.ValidatorSet()

	m.Ibft.transport = m

	return m
}

func newMockIBFTWithMockBlockchain(
	t *testing.T,
	pool *testerAccountPool,
	mockBlockchain *MockBlockchain,
	account string,
) *mockIbft {
	t.Helper()

	m := &mockIbft{
		t:          t,
		pool:       pool,
		blockchain: mockBlockchain,
		respMsg:    []*proto.MessageReq{},
	}

	var addr *testerAccount

	if account == "" {
		// account not in validator set, create a new one that is not part
		// of the genesis
		pool.add("xx")
		addr = pool.get("xx")
	} else {
		addr = pool.get(account)
	}

	ibft := &Ibft{
		logger:           hclog.NewNullLogger(),
		config:           &consensus.Config{},
		blockchain:       m,
		validatorKey:     addr.priv,
		validatorKeyAddr: addr.Address(),
		closeCh:          make(chan struct{}),
		isClosed:         atomic.NewBool(false),
		updateCh:         make(chan struct{}),
		operator:         &operator{},
		state:            newState(),
		epochSize:        DefaultEpochSize,
		metrics:          consensus.NilMetrics(),
	}

	initIbftMechanism(PoA, ibft)

	// by default set the state to (1, 0)
	ibft.state.view = proto.ViewMsg(1, 0)

	m.Ibft = ibft

	assert.NoError(t, ibft.setupSnapshot())
	assert.NoError(t, ibft.createKey())

	// set the initial validators frrom the snapshot
	ibft.state.validators = pool.ValidatorSet()

	m.Ibft.transport = m

	return m
}

type expectResult struct {
	state    IbftState
	sequence uint64
	round    uint64
	locked   bool
	err      error

	// num of messages
	prepareMsgs uint64
	commitMsgs  uint64

	// outgoing messages
	outgoing uint64
}

func (m *mockIbft) expect(res expectResult) {
	if sequence := m.state.view.Sequence; sequence != res.sequence {
		m.t.Fatalf("incorrect sequence got=%d expected=%d", sequence, res.sequence)
	}

	if round := m.state.view.Round; round != res.round {
		m.t.Fatalf("incorrect round got=%d expected=%d", round, res.round)
	}

	if m.getState() != res.state {
		m.t.Fatalf("incorrect state got=%s expected=%s", m.getState(), res.state)
	}

	if size := len(m.state.prepared); uint64(size) != res.prepareMsgs {
		m.t.Fatalf("incorrect prepared messages got=%d expected=%d", size, res.prepareMsgs)
	}

	if size := len(m.state.committed); uint64(size) != res.commitMsgs {
		m.t.Fatalf("incorrect commit messages got=%d expected=%d", size, res.commitMsgs)
	}

	if m.state.locked != res.locked {
		m.t.Fatalf("incorrect locked got=%v expected=%v", m.state.locked, res.locked)
	}

	if size := len(m.respMsg); uint64(size) != res.outgoing {
		m.t.Fatalf("incorrect outgoing messages got=%v expected=%v", size, res.outgoing)
	}

	if !errors.Is(m.state.err, res.err) {
		m.t.Fatalf("incorrect error got=%v expected=%v", m.state.err, res.err)
	}
}

var (
	hookTypes = []HookType{
		VerifyHeadersHook,
		ProcessHeadersHook,
		InsertBlockHook,
		CandidateVoteHook,
		AcceptStateLogHook,
		VerifyBlockHook,
		PreStateCommitHook,
	}
)

type mockMechanism struct {
	BaseConsensusMechanism
	fired                   map[HookType]uint
	isAvailable             func(HookType, uint64) bool
	shouldWriteTransactions func(uint64) bool
}

func newMockMechanism(t *testing.T, i *Ibft, params *IBFTFork) *mockMechanism {
	t.Helper()

	m := &mockMechanism{
		BaseConsensusMechanism: BaseConsensusMechanism{
			mechanismType: PoA,
			ibft:          i,
		},
		fired: make(map[HookType]uint),
	}

	if err := m.initializeParams(params); err != nil {
		t.Fatal(err)
	}

	m.initializeHookMap()

	return m
}

func (m *mockMechanism) IsAvailable(hook HookType, height uint64) bool {
	if m.isAvailable != nil {
		return m.isAvailable(hook, height)
	}

	return true
}

func (m *mockMechanism) ShouldWriteTransactions(height uint64) bool {
	if m.shouldWriteTransactions != nil {
		return m.shouldWriteTransactions(height)
	}

	return true
}

func (m *mockMechanism) initializeHookMap() {
	// Create the hook map
	m.hookMap = make(map[HookType]func(interface{}) error)

	for _, hook := range hookTypes {
		// reassign as local variable
		hook := hook

		m.hookMap[hook] = func(_param interface{}) error {
			m.fired[hook]++

			return nil
		}
	}
}

func (m *mockMechanism) resetFiredCount() {
	m.fired = make(map[HookType]uint)
}

func Test_runHook(t *testing.T) {
	i := newMockIbft(t, []string{"A", "B", "C", "D"}, "A")
	mockMechanism := newMockMechanism(t, i.Ibft, &IBFTFork{
		Type: PoA,
		From: common.JSONNumber{Value: 0},
	})
	i.mechanisms = []ConsensusMechanism{mockMechanism}

	for _, hook := range hookTypes {
		name := fmt.Sprintf("IBFT should emit %s", string(hook))
		t.Run(name, func(t *testing.T) {
			mockMechanism.resetFiredCount()

			assert.NoError(t, i.runHook(hook, 0, nil))

			for _, h := range hookTypes {
				shouldReceive := h == hook
				received := mockMechanism.fired[h] > 0

				// check fired or not
				if received != shouldReceive {
					if shouldReceive {
						t.Fatalf("should fire %s hook but it was not called", string(h))
					} else {
						t.Fatalf("shouldn't fire %s hook but it was called", string(h))
					}
				}

				// check fired only one time
				if shouldReceive && received {
					assert.Equalf(t,
						uint(1),
						mockMechanism.fired[h],
						"should fire %s hook only once, but it was called multiple times",
						string(h),
					)
				}
			}
		})
	}
}

func Test_shouldWriteTransactions(t *testing.T) {
	tests := []struct {
		name                    string
		mechanismShouldWriteTxs []func(uint64) bool
		res                     bool
	}{
		{
			name: "should return true because all mechanisms returns true",
			mechanismShouldWriteTxs: []func(uint64) bool{
				func(_h uint64) bool {
					return true
				},
				func(_h uint64) bool {
					return true
				},
			},
			res: true,
		},
		{
			name: "should return false because all mechanisms return false",
			mechanismShouldWriteTxs: []func(uint64) bool{
				func(_h uint64) bool {
					return false
				},
				func(_h uint64) bool {
					return false
				},
			},
			res: false,
		},
		{
			name: "should return true because one of the mechanisms return true",
			mechanismShouldWriteTxs: []func(uint64) bool{
				func(_h uint64) bool {
					return true
				},
				func(_h uint64) bool {
					return false
				},
			},
			res: true,
		},
	}

	i := newMockIbft(t, []string{"A", "B", "C", "D"}, "A")
	mockMechanism1 := newMockMechanism(t, i.Ibft, &IBFTFork{
		Type: PoA,
		From: common.JSONNumber{Value: 0},
	})
	mockMechanism2 := newMockMechanism(t, i.Ibft, &IBFTFork{
		Type: PoA,
		From: common.JSONNumber{Value: 0},
	})
	i.mechanisms = []ConsensusMechanism{mockMechanism1, mockMechanism2}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			mockMechanism1.shouldWriteTransactions = testcase.mechanismShouldWriteTxs[0]
			mockMechanism2.shouldWriteTransactions = testcase.mechanismShouldWriteTxs[1]

			res := i.shouldWriteTransactions(0)
			assert.Equalf(t, testcase.res, res, "shouldWriteTransactions should return %t", testcase.res)
		})
	}
}

func TestBaseConsensusMechanismIsInRange(t *testing.T) {
	tests := []struct {
		name string
		// mechanism field
		from common.JSONNumber
		to   *common.JSONNumber
		// arg
		height uint64
		// expected
		res bool
	}{
		{
			name:   "should return true if the given height equals to from and to is not set",
			from:   common.JSONNumber{Value: 0},
			to:     nil,
			height: 0,
			res:    true,
		},
		{
			name:   "should return true if the given height is greater than from and to is not set",
			from:   common.JSONNumber{Value: 0},
			to:     nil,
			height: 10,
			res:    true,
		},
		{
			name:   "should return true if the given height equals to from and less than to",
			from:   common.JSONNumber{Value: 0},
			to:     &common.JSONNumber{Value: 10},
			height: 0,
			res:    true,
		},
		{
			name:   "should return true if the given height is grater than from and less than to",
			from:   common.JSONNumber{Value: 0},
			to:     &common.JSONNumber{Value: 10},
			height: 5,
			res:    true,
		},
		{
			name:   "should return true if the given height is grater than from and equals to to",
			from:   common.JSONNumber{Value: 0},
			to:     &common.JSONNumber{Value: 10},
			height: 10,
			res:    true,
		},
		{
			name:   "should return false if the given height is less than from and to",
			from:   common.JSONNumber{Value: 5},
			to:     &common.JSONNumber{Value: 10},
			height: 0,
			res:    false,
		},
		{
			name:   "should return false if the given height is greater than from and to",
			from:   common.JSONNumber{Value: 5},
			to:     &common.JSONNumber{Value: 10},
			height: 15,
			res:    false,
		},
	}

	i := newMockIbft(t, []string{"A", "B", "C", "D"}, "A")

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			mockMechanism := newMockMechanism(t, i.Ibft, &IBFTFork{
				Type: PoA,
				From: testcase.from,
				To:   testcase.to,
			})
			i.mechanisms = []ConsensusMechanism{mockMechanism}

			res := mockMechanism.IsInRange(testcase.height)

			assert.Equal(t, testcase.res, res)
		})
	}
}

func TestGetIBFTForks(t *testing.T) {
	tests := []struct {
		name       string
		ibftConfig map[string]interface{}
		forks      []IBFTFork
		err        error
	}{
		{
			name: "should return a IBFTFork when ibftConfig has type",
			ibftConfig: map[string]interface{}{
				"type": string(PoA),
			},
			forks: []IBFTFork{
				{
					Type: PoA,
					From: common.JSONNumber{Value: 0},
				},
			},
			err: nil,
		},
		{
			name: "should return multiple IBFTForks when ibftConfig has types",
			ibftConfig: map[string]interface{}{
				"types": []interface{}{
					map[string]interface{}{
						"type": PoA,
						"from": 0,
						"to":   100,
					},
					map[string]interface{}{
						"type":       PoS,
						"deployment": "0x32", // 50
						"from":       "0x65", // 101
					},
				},
			},
			forks: []IBFTFork{
				{
					Type: PoA,
					From: common.JSONNumber{Value: 0},
					To:   &common.JSONNumber{Value: 100},
				},
				{
					Type:       PoS,
					Deployment: &common.JSONNumber{Value: 50},
					From:       common.JSONNumber{Value: 101},
				},
			},
			err: nil,
		},
		{
			name: "should return error if neither type and types is not set",
			ibftConfig: map[string]interface{}{
				"foo": string(PoA),
			},
			forks: nil,
			err:   errors.New("current IBFT type not found"),
		},
	}

	for _, testcase := range tests {
		t.Run(testcase.name, func(t *testing.T) {
			forks, err := GetIBFTForks(testcase.ibftConfig)
			assert.Equal(t, testcase.forks, forks)
			assert.Equal(t, testcase.err, err)
		})
	}
}
