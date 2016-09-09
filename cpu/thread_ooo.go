package cpu

import "github.com/mcai/acogo/cpu/regs"

type OoOThread struct {
	*MemoryHierarchyThread

	BranchPredictor                        *BranchPredictor

	IntPhysicalRegs                        *PhysicalRegisterFile
	FpPhysicalRegs                         *PhysicalRegisterFile
	MiscPhysicalRegs                       *PhysicalRegisterFile

	RenameTable                            map[*RegisterDependency]*PhysicalRegister

	DecodeBuffer                           *PipelineBuffer
	ReorderBuffer                          *PipelineBuffer
	LoadStoreQueue                         *PipelineBuffer

	FetchNpc                               uint32
	FetchNnpc                              uint32

	lastDecodedDynamicInst                 *DynamicInst
	lastDecodedDynamicInstCommitted        bool

	LastCommitCycle                        int64
	NoDynamicInstCommittedCounterThreshold uint32
}

func NewOoOThread(core Core, num int32) *OoOThread {
	var thread = &OoOThread{
		MemoryHierarchyThread:NewMemoryHierarchyThread(core, num),

		BranchPredictor:NewBranchPredictor(
			core.Processor().Experiment.CPUConfig.BranchPredictorSize,
			core.Processor().Experiment.CPUConfig.BranchTargetBufferNumSets,
			core.Processor().Experiment.CPUConfig.BranchTargetBufferAssoc,
			core.Processor().Experiment.CPUConfig.ReturnAddressStackSize,
		),

		IntPhysicalRegs:NewPhysicalRegisterFile(core.Processor().Experiment.CPUConfig.PhysicalRegisterFileSize),
		FpPhysicalRegs:NewPhysicalRegisterFile(core.Processor().Experiment.CPUConfig.PhysicalRegisterFileSize),
		MiscPhysicalRegs:NewPhysicalRegisterFile(core.Processor().Experiment.CPUConfig.PhysicalRegisterFileSize),

		RenameTable:make(map[*RegisterDependency]*PhysicalRegister),

		DecodeBuffer:NewPipelineBuffer(core.Processor().Experiment.CPUConfig.DecodeBufferSize),
		ReorderBuffer:NewPipelineBuffer(core.Processor().Experiment.CPUConfig.ReorderBufferSize),
		LoadStoreQueue:NewPipelineBuffer(core.Processor().Experiment.CPUConfig.LoadStoreQueueSize),
	}

	for i := uint32(0); i < regs.NUM_INT_REGISTERS; i++ {
		var dependency = NewRegisterDependency(RegisterDependencyType_INT, i)
		var physicalReg = thread.IntPhysicalRegs.PhysicalRegisters[i]
		physicalReg.Reserve(dependency)
		thread.RenameTable[dependency] = physicalReg
	}

	for i := uint32(0); i < regs.NUM_FP_REGISTERS; i++ {
		var dependency = NewRegisterDependency(RegisterDependencyType_FP, i)
		var physicalReg = thread.FpPhysicalRegs.PhysicalRegisters[i]
		physicalReg.Reserve(dependency)
		thread.RenameTable[dependency] = physicalReg
	}

	for i := uint32(0); i < regs.NUM_MISC_REGISTERS; i++ {
		var dependency = NewRegisterDependency(RegisterDependencyType_MISC, i)
		var physicalReg = thread.MiscPhysicalRegs.PhysicalRegisters[i]
		physicalReg.Reserve(dependency)
		thread.RenameTable[dependency] = physicalReg
	}

	return thread
}

func (thread *OoOThread) UpdateFetchNpcAndNnpcFromRegs() {
	thread.FetchNpc = thread.Context().Regs().Npc
	thread.FetchNnpc = thread.Context().Regs().Nnpc

	thread.LastCommitCycle = thread.Core().Processor().Experiment.CycleAccurateEventQueue().CurrentCycle
}

func (thread *OoOThread) CanFetch() bool {
	if thread.FetchStalled {
		return false
	}

	var cacheLineToFetch = thread.Core().L1IController().Cache.GetTag(thread.FetchNpc)
	if int32(cacheLineToFetch) != thread.LastFetchedCacheLine {
		if !thread.Core().CanIfetch(thread, thread.FetchNpc) {
			return false
		} else {
			thread.Core().Ifetch(thread, thread.FetchNpc, thread.FetchNpc, func() {
				thread.FetchStalled = false
			})

			thread.FetchStalled = true
			thread.LastFetchedCacheLine = int32(cacheLineToFetch)

			return false
		}
	}

	return true
}

func (thread *OoOThread) Fetch() {
	if !thread.CanFetch() {
		return
	}

	var hasDone = false

	for !hasDone {
		if thread.Context().State != ContextState_RUNNING {
			break
		}

		if thread.DecodeBuffer.Full() {
			break
		}

		if thread.Context().Regs().Npc != thread.FetchNpc {
			if thread.Context().Speculative {
				thread.Context().Regs().Npc = thread.FetchNpc
			} else {
				thread.Context().EnterSpeculativeState()
			}
		}

		var dynamicInst *DynamicInst

		for {
			var staticInst = thread.Context().DecodeNextStaticInst()

			dynamicInst = NewDynamicInst(thread, thread.Context().Regs().Pc, staticInst)

			staticInst.Execute(thread.Context())

			if dynamicInst.StaticInst.Mnemonic.StaticInstType == StaticInstType_NOP {
				thread.UpdateFetchNpcAndNnpcFromRegs()
			}

			if dynamicInst.StaticInst.Mnemonic.StaticInstType != StaticInstType_NOP {
				break
			}
		}

		thread.FetchNpc = thread.FetchNnpc

		if !thread.Context().Speculative && thread.Context().State != ContextState_RUNNING {
			thread.lastDecodedDynamicInst = dynamicInst
			thread.lastDecodedDynamicInstCommitted = false
		}

		if (thread.FetchNpc + 4) % thread.Core().L1IController().Cache.LineSize() == 0 {
			hasDone = true
		}

		var branchPredictorUpdate = NewBranchPredictorUpdate()

		var returnAddressStackRecoverTop uint32

		if dynamicInst.StaticInst.Mnemonic.StaticInstType.IsControl() {
			thread.FetchNnpc, returnAddressStackRecoverTop = thread.BranchPredictor.Predict(thread.FetchNpc, dynamicInst.StaticInst.Mnemonic, branchPredictorUpdate)
		} else {
			thread.FetchNnpc, returnAddressStackRecoverTop = thread.FetchNpc + 4, 0
		}

		if thread.FetchNnpc != thread.FetchNpc + 4 {
			hasDone = true
		}

		thread.DecodeBuffer.Entries = append(
			thread.DecodeBuffer.Entries,
			NewDecodeBufferEntry(
				dynamicInst,
				thread.Context().Regs().Npc,
				thread.Context().Regs().Nnpc,
				thread.FetchNnpc,
				returnAddressStackRecoverTop,
				branchPredictorUpdate,
				thread.Context().Speculative,
			),
		)
	}
}

func (thread *OoOThread) RegisterRenameOne() bool {
	var decodeBufferEntry = thread.DecodeBuffer.Entries[0].(*DecodeBufferEntry)

	var dynamicInst = decodeBufferEntry.DynamicInst

	for outputDependencyType, numPhysicalRegistersToAllocate := range dynamicInst.StaticInst.NumPhysicalRegistersToAllocate {
		if thread.GetPhysicalRegisterFile(outputDependencyType).NumFreePhysicalRegisters < numPhysicalRegistersToAllocate {
			return false
		}
	}

	if dynamicInst.StaticInst.Mnemonic.StaticInstType.IsLoadOrStore() && thread.LoadStoreQueue.Full() {
		return false
	}

	var reorderBufferEntry = NewReorderBufferEntry(
		thread,
		dynamicInst,
		decodeBufferEntry.Npc,
		decodeBufferEntry.Nnpc,
		decodeBufferEntry.PredictedNnpc,
		decodeBufferEntry.ReturnAddressStackRecoverTop,
		decodeBufferEntry.BranchPredictorUpdate,
		decodeBufferEntry.Speculative,
	)

	reorderBufferEntry.EffectiveAddressComputation = dynamicInst.StaticInst.Mnemonic.StaticInstType.IsLoadOrStore()

	for _, inputDependency := range dynamicInst.StaticInst.InputDependencies {
		reorderBufferEntry.SourcePhysicalRegisters()[inputDependency] = thread.RenameTable[inputDependency]
	}

	for _, outputDependency := range dynamicInst.StaticInst.OutputDependencies {
		reorderBufferEntry.OldPhysicalRegisters()[outputDependency] = thread.RenameTable[outputDependency]
		var physReg = thread.GetPhysicalRegisterFile(outputDependency.DependencyType).Allocate(outputDependency)
		thread.RenameTable[outputDependency] = physReg
		reorderBufferEntry.TargetPhysicalRegisters()[outputDependency] = physReg
	}

	for _, sourcePhysReg := range reorderBufferEntry.SourcePhysicalRegisters() {
		if !sourcePhysReg.Ready() {
			reorderBufferEntry.SetNumNotReadyOperands(reorderBufferEntry.NumNotReadyOperands() + 1)
			sourcePhysReg.Dependents = append(sourcePhysReg.Dependents, reorderBufferEntry)
		}
	}

	if reorderBufferEntry.EffectiveAddressComputation {
		var physReg = reorderBufferEntry.SourcePhysicalRegisters()[dynamicInst.StaticInst.InputDependencies[0]]

		if !physReg.Ready() {
			physReg.EffectiveAddressComputationOperandDependents = append(
				physReg.EffectiveAddressComputationOperandDependents,
				reorderBufferEntry,
			)
		} else {
			reorderBufferEntry.EffectiveAddressComputationOperandReady = true
		}
	}

	if dynamicInst.StaticInst.Mnemonic.StaticInstType.IsLoadOrStore() {
		var loadStoreQueueEntry = NewLoadStoreQueueEntry(
			thread,
			dynamicInst,
			decodeBufferEntry.Npc,
			decodeBufferEntry.Nnpc,
			decodeBufferEntry.PredictedNnpc,
			0,
			nil,
			false,
		)

		loadStoreQueueEntry.EffectiveAddress = dynamicInst.EffectiveAddress

		loadStoreQueueEntry.SetSourcePhysicalRegisters(reorderBufferEntry.SourcePhysicalRegisters())
		loadStoreQueueEntry.SetTargetPhysicalRegisters(reorderBufferEntry.TargetPhysicalRegisters())

		for _, sourcePhysReg := range loadStoreQueueEntry.SourcePhysicalRegisters() {
			if !sourcePhysReg.Ready() {
				sourcePhysReg.Dependents = append(sourcePhysReg.Dependents, loadStoreQueueEntry)
			}
		}

		loadStoreQueueEntry.SetNumNotReadyOperands(reorderBufferEntry.NumNotReadyOperands())

		var storeAddressPhysReg = loadStoreQueueEntry.SourcePhysicalRegisters()[dynamicInst.StaticInst.InputDependencies[0]]
		if !storeAddressPhysReg.Ready() {
			storeAddressPhysReg.StoreAddressDependents = append(
				storeAddressPhysReg.StoreAddressDependents,
				loadStoreQueueEntry,
			)
		} else {
			loadStoreQueueEntry.StoreAddressReady = true
		}

		thread.LoadStoreQueue.Entries = append(thread.LoadStoreQueue.Entries, loadStoreQueueEntry)

		reorderBufferEntry.LoadStoreBufferEntry = loadStoreQueueEntry
	}

	thread.ReorderBuffer.Entries = append(thread.ReorderBuffer.Entries, reorderBufferEntry)

	thread.DecodeBuffer.Entries = thread.DecodeBuffer.Entries[:len(thread.DecodeBuffer.Entries) - 1]

	return true
}

func (thread *OoOThread) DispatchOne() bool {
	for _, entry := range thread.ReorderBuffer.Entries {
		var reorderBufferEntry = entry.(*ReorderBufferEntry)

		if !reorderBufferEntry.Dispatched() {
			if reorderBufferEntry.AllOperandReady() {
				thread.Core().SetReadyInstructionQueue(
					append(
						thread.Core().ReadyInstructionQueue(),
						reorderBufferEntry,
					),
				)
			} else {
				thread.Core().SetWaitingInstructionQueue(
					append(
						thread.Core().WaitingInstructionQueue(),
						reorderBufferEntry,
					),
				)
			}

			reorderBufferEntry.SetDispatched(true)

			if reorderBufferEntry.LoadStoreBufferEntry != nil {
				var loadStoreQueueEntry = reorderBufferEntry.LoadStoreBufferEntry

				if loadStoreQueueEntry.DynamicInst().StaticInst.Mnemonic.StaticInstType == StaticInstType_ST {
					if loadStoreQueueEntry.AllOperandReady() {
						thread.Core().SetReadyStoreQueue(
							append(
								thread.Core().ReadyStoreQueue(),
								loadStoreQueueEntry,
							),
						)
					} else {
						thread.Core().SetWaitingStoreQueue(
							append(
								thread.Core().WaitingStoreQueue(),
								loadStoreQueueEntry,
							),
						)
					}
				}

				loadStoreQueueEntry.SetDispatched(true)
			}

			return true
		}
	}

	return false
}

func (thread *OoOThread) RefreshLoadStoreQueue() {
	var stdUnknowns []int32

	for _, entry := range thread.LoadStoreQueue.Entries {
		var loadStoreQueueEntry = entry.(*LoadStoreQueueEntry)

		if loadStoreQueueEntry.DynamicInst().StaticInst.Mnemonic.StaticInstType == StaticInstType_ST {
			if loadStoreQueueEntry.StoreAddressReady {
				break
			} else if !loadStoreQueueEntry.AllOperandReady() {
				stdUnknowns = append(stdUnknowns, loadStoreQueueEntry.EffectiveAddress)
			} else {
				for i, stdUnknown := range stdUnknowns {
					if stdUnknown == loadStoreQueueEntry.EffectiveAddress {
						stdUnknowns[i] = -1
					}
				}
			}
		}

		if loadStoreQueueEntry.DynamicInst().StaticInst.Mnemonic.StaticInstType == StaticInstType_LD &&
			loadStoreQueueEntry.Dispatched() &&
			!loadStoreQueueEntry.Issued() &&
			!loadStoreQueueEntry.Completed() &&
			loadStoreQueueEntry.AllOperandReady() {
			var foundInReadyLoadQueue bool

			for _, readyLoad := range thread.Core().ReadyLoadQueue() {
				if readyLoad == loadStoreQueueEntry {
					foundInReadyLoadQueue = true
					break
				}
			}

			var foundInStdUnknowns bool

			for _, stdUnknown := range stdUnknowns {
				if stdUnknown == loadStoreQueueEntry.EffectiveAddress {
					foundInStdUnknowns = true
					break
				}
			}

			if !foundInReadyLoadQueue && !foundInStdUnknowns {
				thread.Core().SetReadyLoadQueue(
					append(
						thread.Core().ReadyLoadQueue(),
						loadStoreQueueEntry,
					),
				)
			}
		}
	}
}

func (thread *OoOThread) Commit() {
	var commitTimeout = int64(1000000)

	if thread.Core().Processor().Experiment.CycleAccurateEventQueue().CurrentCycle - thread.LastCommitCycle > commitTimeout {
		if thread.NoDynamicInstCommittedCounterThreshold > 5 {
			thread.Core().Processor().Experiment.MemoryHierarchy.DumpPendingFlowTree()
		} else {
			thread.LastCommitCycle = thread.Core().Processor().Experiment.CycleAccurateEventQueue().CurrentCycle
			thread.NoDynamicInstCommittedCounterThreshold++
		}
	}

	var numCommitted = uint32(0)

	for !thread.ReorderBuffer.Empty() && numCommitted < thread.Core().Processor().Experiment.CPUConfig.CommitWidth {
		var reorderBufferEntry = thread.ReorderBuffer.Entries[0].(*ReorderBufferEntry)

		if !reorderBufferEntry.Completed() {
			break
		}

		if reorderBufferEntry.Speculative() {
			thread.BranchPredictor.ReturnAddressStack.Recover(reorderBufferEntry.ReturnAddressStackRecoverTop())

			thread.Context().ExitSpeculativeState()

			thread.FetchNpc = thread.Context().Regs().Npc
			thread.FetchNnpc = thread.Context().Regs().Nnpc

			thread.Squash()
			break
		}

		if reorderBufferEntry.EffectiveAddressComputation {
			var loadStoreQueueEntry = reorderBufferEntry.LoadStoreBufferEntry

			if !loadStoreQueueEntry.Completed() {
				break
			}

			thread.Core().RemoveFromQueues(loadStoreQueueEntry)

			thread.removeFromLoadStoreQueue(loadStoreQueueEntry)
		}

		for _, outputDependency := range reorderBufferEntry.DynamicInst().StaticInst.OutputDependencies {
			if outputDependency.ToInt() != 0 {
				reorderBufferEntry.OldPhysicalRegisters()[outputDependency].Reclaim()
				reorderBufferEntry.TargetPhysicalRegisters()[outputDependency].Commit()
			}
		}

		if reorderBufferEntry.DynamicInst().StaticInst.Mnemonic.StaticInstType.IsControl() {
			thread.BranchPredictor.Update(
				reorderBufferEntry.DynamicInst().Pc,
				reorderBufferEntry.Nnpc(),
				reorderBufferEntry.Nnpc() != reorderBufferEntry.Npc() + 4,
				reorderBufferEntry.PredictedNnpc() == reorderBufferEntry.Nnpc(),
				reorderBufferEntry.DynamicInst().StaticInst.Mnemonic,
				reorderBufferEntry.BranchPredictorUpdate(),
			)
		}

		thread.Core().RemoveFromQueues(reorderBufferEntry)

		if thread.Context().State == ContextState_FINISHED && reorderBufferEntry.DynamicInst() == thread.lastDecodedDynamicInst {
			thread.lastDecodedDynamicInstCommitted = true
		}

		thread.ReorderBuffer.Entries = thread.ReorderBuffer.Entries[1:]

		thread.numDynamicInsts++

		thread.LastCommitCycle = thread.Core().Processor().Experiment.CycleAccurateEventQueue().CurrentCycle

		numCommitted++
	}
}

func (thread *OoOThread) removeFromLoadStoreQueue(entryToRemove *LoadStoreQueueEntry) {
	var loadStoreQueueEntriesToReserve []interface{}

	for _, entry := range thread.LoadStoreQueue.Entries {
		if entry != entryToRemove {
			loadStoreQueueEntriesToReserve = append(loadStoreQueueEntriesToReserve, entry)
		}
	}

	thread.LoadStoreQueue.Entries = loadStoreQueueEntriesToReserve
}

func (thread *OoOThread) Squash() {
	for !thread.ReorderBuffer.Empty() {
		var reorderBufferEntry = thread.ReorderBuffer.Entries[len(thread.ReorderBuffer.Entries) - 1].(*ReorderBufferEntry)

		if reorderBufferEntry.EffectiveAddressComputation {
			var loadStoreQueueEntry = reorderBufferEntry.LoadStoreBufferEntry

			thread.Core().RemoveFromQueues(loadStoreQueueEntry)

			thread.removeFromLoadStoreQueue(loadStoreQueueEntry)
		}

		thread.Core().RemoveFromQueues(reorderBufferEntry)

		for _, outputDependency := range reorderBufferEntry.DynamicInst().StaticInst.OutputDependencies {
			if outputDependency.ToInt() != 0 {
				reorderBufferEntry.TargetPhysicalRegisters()[outputDependency].Recover()
				thread.RenameTable[outputDependency] = reorderBufferEntry.OldPhysicalRegisters()[outputDependency]
			}
		}

		reorderBufferEntry.SetTargetPhysicalRegisters(make(map[*RegisterDependency]*PhysicalRegister))

		thread.ReorderBuffer.Entries = thread.ReorderBuffer.Entries[:len(thread.ReorderBuffer.Entries) - 1]
	}

	if !thread.ReorderBuffer.Empty() || !thread.LoadStoreQueue.Empty() {
		panic("Impossible")
	}

	thread.Core().FUPool().ReleaseAll()

	thread.DecodeBuffer.Entries = []interface{}{}
}

func (thread *OoOThread) IsLastDecodedDynamicInstCommitted() bool {
	return thread.lastDecodedDynamicInst == nil || thread.lastDecodedDynamicInstCommitted
}

func (thread *OoOThread) GetPhysicalRegisterFile(dependencyType RegisterDependencyType) *PhysicalRegisterFile {
	switch dependencyType {
	case RegisterDependencyType_INT:
		return thread.IntPhysicalRegs
	case RegisterDependencyType_FP:
		return thread.FpPhysicalRegs
	case RegisterDependencyType_MISC:
		return thread.MiscPhysicalRegs
	default:
		panic("Impossible")
	}
}