package ssaUtils

import (
	"StaticRaceDetector/domain"
	"golang.org/x/tools/go/ssa"
)

type CFG struct {
	ComputedBlockIDsToSummaries  map[int]*domain.FunctionState
	lastBlock                    *ssa.BasicBlock
	getSummary                   func(goroutineState *domain.GoroutineState, block *ssa.BasicBlock) *domain.FunctionState
	calculateMergedBranchesState func(blocks []*ssa.BasicBlock) *domain.FunctionState
	getNextBlocks                func(block *ssa.BasicBlock) []*ssa.BasicBlock
	getPreviousBlocks            func(block *ssa.BasicBlock) []*ssa.BasicBlock

	DeferredFunctions map[int][]*domain.DeferFunction
}

func newCFG() *CFG {
	return &CFG{
		ComputedBlockIDsToSummaries: make(map[int]*domain.FunctionState, 0),
	}
}

func (cfg *CFG) AreAllPrecedingCalculated(precedingBlocks []*ssa.BasicBlock) bool {
	for _, block := range precedingBlocks {
		_, isExist := cfg.ComputedBlockIDsToSummaries[block.Index]
		if !isExist {
			return false
		}
	}
	return true
}

func (cfg *CFG) mergeSuccsBlocks(blocks []*ssa.BasicBlock) *domain.FunctionState {
	blocksLen := len(blocks)
	state := cfg.ComputedBlockIDsToSummaries[blocks[blocksLen-1].Index].Copy()
	for i := blocksLen - 2; i >= 0; i-- {
		predBlockSummary := cfg.ComputedBlockIDsToSummaries[blocks[i].Index].Copy()
		state.MergeBranchState(predBlockSummary)
	}
	return state
}

func (cfg *CFG) mergePredBlocks(blocks []*ssa.BasicBlock) *domain.FunctionState {
	state := cfg.ComputedBlockIDsToSummaries[blocks[0].Index].Copy()
	for _, predBlock := range blocks[1:] {
		predBlockSummary := cfg.ComputedBlockIDsToSummaries[predBlock.Index].Copy()
		state.MergeBranchState(predBlockSummary)
	}
	return state
}

func CalculateBlocks(GoroutineState *domain.GoroutineState, startBlock *ssa.BasicBlock) (*domain.FunctionState, *ssa.BasicBlock) {
	cfgDown := newCFG()
	cfgDown.getSummary = func(goroutineState *domain.GoroutineState, block *ssa.BasicBlock) *domain.FunctionState {
		return GetBlockSummary(goroutineState, block)
	}
	cfgDown.calculateMergedBranchesState = func(blocks []*ssa.BasicBlock) *domain.FunctionState {
		return cfgDown.mergePredBlocks(blocks)
	}
	cfgDown.getNextBlocks = func(block *ssa.BasicBlock) []*ssa.BasicBlock {
		return block.Succs
	}
	cfgDown.getPreviousBlocks = func(block *ssa.BasicBlock) []*ssa.BasicBlock {
		return block.Preds
	}
	cfgDown.traverseGraph(GoroutineState, startBlock)
	funcState := cfgDown.ComputedBlockIDsToSummaries[cfgDown.lastBlock.Index]
	return funcState, cfgDown.lastBlock
}

func CalculateDefers(GoroutineState *domain.GoroutineState, startBlock *ssa.BasicBlock, deferredFunctions []*domain.DeferFunction) *domain.FunctionState {
	deferredMap := make(map[int][]*domain.DeferFunction, 0)
	for _, block := range deferredFunctions {
		deferredMap[block.BlockIndex] = append(deferredMap[block.BlockIndex], block)
	}

	cfgUp := newCFG()
	cfgUp.DeferredFunctions = deferredMap
	cfgUp.getSummary = func(goroutineState *domain.GoroutineState, block *ssa.BasicBlock) *domain.FunctionState {
		return cfgUp.runDefers(goroutineState, block)
	}
	cfgUp.calculateMergedBranchesState = func(blocks []*ssa.BasicBlock) *domain.FunctionState {
		return cfgUp.mergeSuccsBlocks(blocks)
	}
	cfgUp.getNextBlocks = func(block *ssa.BasicBlock) []*ssa.BasicBlock {
		return block.Preds
	}
	cfgUp.getPreviousBlocks = func(block *ssa.BasicBlock) []*ssa.BasicBlock {
		return block.Succs
	}
	cfgUp.traverseGraph(GoroutineState, startBlock)
	funcState := cfgUp.ComputedBlockIDsToSummaries[cfgUp.lastBlock.Index]
	return funcState
}

func (cfg *CFG) traverseGraph(goroutineState *domain.GoroutineState, block *ssa.BasicBlock) {
	nextBlocks := cfg.getNextBlocks(block)
	prevBlocks := cfg.getPreviousBlocks(block)

	// When 2 path diverge, shared blocks are traversed again. In that case we return since already calculated the summary for that block.
	_, wasCalculated := cfg.ComputedBlockIDsToSummaries[block.Index]
	if wasCalculated {
		return
	}

	// We can merge only once all preceding blocks were calculated. If one of the preceding blocks wasn't calculated yet, we return and we'll reach this block again once all preceding blocks are calculated.
	if !cfg.AreAllPrecedingCalculated(prevBlocks) {
		return
	}

	var calculatedState *domain.FunctionState
	currBlockState := cfg.getSummary(goroutineState, block)
	if len(prevBlocks) > 0 {
		calculatedState = cfg.calculateMergedBranchesState(prevBlocks)
		currBlockState.UpdateGuardedAccessesWithLockset(calculatedState.Lockset)
		calculatedState.MergeStates(currBlockState)
	} else {
		calculatedState = currBlockState
	}

	cfg.ComputedBlockIDsToSummaries[block.Index] = calculatedState

	if len(nextBlocks) == 0 {
		cfg.lastBlock = block
		return
	}

	for _, blockToExecute := range nextBlocks {
		cfg.traverseGraph(goroutineState, blockToExecute)
	}
}