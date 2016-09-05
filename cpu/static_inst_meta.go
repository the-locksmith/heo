package cpu

import "github.com/mcai/acogo/cpu/regs"

type StaticInstFlag string

const (
	StaticInstFlag_INT_COMP = StaticInstFlag("INT_COMP")
	StaticInstFlag_FP_COMP = StaticInstFlag("FP_COMP")
	StaticInstFlag_UNCOND = StaticInstFlag("UNCOND")
	StaticInstFlag_COND = StaticInstFlag("COND")
	StaticInstFlag_LD = StaticInstFlag("LD")
	StaticInstFlag_ST = StaticInstFlag("ST")
	StaticInstFlag_DIRECT_JMP = StaticInstFlag("DIRECT_JUMP")
	StaticInstFlag_INDIRECT_JMP = StaticInstFlag("INDIRECT_JUMP")
	StaticInstFlag_FUNC_CALL = StaticInstFlag("FUNC_CALL")
	StaticInstFlag_FUNC_RET = StaticInstFlag("FUNC_RET")
	StaticInstFlag_IMM = StaticInstFlag("IMM")
	StaticInstFlag_DISPLACED_ADDRESSING = StaticInstFlag("DISPLACED_ADDRESSING")
	StaticInstFlag_TRAP = StaticInstFlag("TRAP")
	StaticInstFlag_NOP = StaticInstFlag("NOP")
)

type StaticInstType string

const (
	StaticInstType_INT_COMP = StaticInstType("INT_COMP")
	StaticInstType_FP_COMP = StaticInstType("FP_COMP")
	StaticInstType_COND = StaticInstType("COND")
	StaticInstType_UNCOND = StaticInstType("UNCOND")
	StaticInstType_LD = StaticInstType("LD")
	StaticInstType_ST = StaticInstType("ST")
	StaticInstType_FUNC_CALL = StaticInstType("FUNC_CALL")
	StaticInstType_FUNC_RET = StaticInstType("FUNC_RET")
	StaticInstType_TRAP = StaticInstType("TRAP")
	StaticInstType_NOP = StaticInstType("NOP")
)

type RegisterDependencyType string

const (
	RegisterDependencyType_INT = RegisterDependencyType("INT")
	RegisterDependencyType_FP = RegisterDependencyType("FP")
	RegisterDependencyType_MISC = RegisterDependencyType("MISC")
)

type RegisterDependency struct {
	DependencyType RegisterDependencyType
	Num            uint32
}

func NewRegisterDependency(dependencyType RegisterDependencyType, num uint32) *RegisterDependency {
	var registerDependency = &RegisterDependency{
		DependencyType:dependencyType,
		Num:num,
	}

	return registerDependency
}

func NewRegisterDependencyFromInt(i uint32) *RegisterDependency {
	if i < regs.NUM_INT_REGISTERS {
		return NewRegisterDependency(RegisterDependencyType_INT, i)
	} else if i < regs.NUM_INT_REGISTERS + regs.NUM_FP_REGISTERS {
		return NewRegisterDependency(RegisterDependencyType_FP, i - regs.NUM_INT_REGISTERS)
	} else {
		return NewRegisterDependency(RegisterDependencyType_MISC, i - regs.NUM_INT_REGISTERS - regs.NUM_FP_REGISTERS)
	}
}

func (registerDependency *RegisterDependency) ToInt() uint32 {
	switch registerDependency.DependencyType {
	case RegisterDependencyType_INT:
		return registerDependency.Num
	case RegisterDependencyType_FP:
		return regs.NUM_INT_REGISTERS + registerDependency.Num
	case RegisterDependencyType_MISC:
		return regs.NUM_INT_REGISTERS + regs.NUM_FP_REGISTERS + registerDependency.Num
	default:
		panic("Impossible")
	}
}

type StaticInstDependency string

const (
	StaticInstDependency_BIT_FIELD_RS = StaticInstDependency("BIT_FIELD_RS")
	StaticInstDependency_BIT_FIELD_RT = StaticInstDependency("BIT_FIELD_RT")
	StaticInstDependency_BIT_FIELD_RD = StaticInstDependency("BIT_FIELD_RD")
	StaticInstDependency_BIT_FIELD_FS = StaticInstDependency("BIT_FIELD_FS")
	StaticInstDependency_BIT_FIELD_FT = StaticInstDependency("BIT_FIELD_FT")
	StaticInstDependency_BIT_FIELD_FD = StaticInstDependency("BIT_FIELD_FD")
	StaticInstDependency_REGISTER_RA = StaticInstDependency("REGISTER_RA")
	StaticInstDependency_REGISTER_V0 = StaticInstDependency("REGISTER_V0")
	StaticInstDependency_REGISTER_HI = StaticInstDependency("REGISTER_HI")
	StaticInstDependency_REGISTER_LO = StaticInstDependency("REGISTER_LO")
	StaticInstDependency_REGISTER_FCSR = StaticInstDependency("REGISTER_FCSR")
)

func (staticInstDependency StaticInstDependency) ToRegisterDependency(machInst MachInst) *RegisterDependency {
	switch staticInstDependency {
	case StaticInstDependency_BIT_FIELD_RS:
		return NewRegisterDependency(RegisterDependencyType_INT, machInst.Rs())
	case StaticInstDependency_BIT_FIELD_RT:
		return NewRegisterDependency(RegisterDependencyType_INT, machInst.Rt())
	case StaticInstDependency_BIT_FIELD_RD:
		return NewRegisterDependency(RegisterDependencyType_INT, machInst.Rd())
	case StaticInstDependency_BIT_FIELD_FS:
		return NewRegisterDependency(RegisterDependencyType_FP, machInst.Fs())
	case StaticInstDependency_BIT_FIELD_FT:
		return NewRegisterDependency(RegisterDependencyType_FP, machInst.Ft())
	case StaticInstDependency_BIT_FIELD_FD:
		return NewRegisterDependency(RegisterDependencyType_FP, machInst.Fd())
	case StaticInstDependency_REGISTER_RA:
		return NewRegisterDependency(RegisterDependencyType_INT, regs.REGISTER_RA)
	case StaticInstDependency_REGISTER_V0:
		return NewRegisterDependency(RegisterDependencyType_INT, regs.REGISTER_V0)
	case StaticInstDependency_REGISTER_HI:
		return NewRegisterDependency(RegisterDependencyType_MISC, regs.REGISTER_HI)
	case StaticInstDependency_REGISTER_LO:
		return NewRegisterDependency(RegisterDependencyType_MISC, regs.REGISTER_LO)
	case StaticInstDependency_REGISTER_FCSR:
		return NewRegisterDependency(RegisterDependencyType_FP, regs.REGISTER_FCSR)
	default:
		panic("Impossible")
	}
}
