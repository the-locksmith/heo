package cpu

type FUType string

const (
	FUType_INT_ALU = FUType("INT_ALU")
	FUType_INT_MULT_DIV = FUType("INT_MULT_DIV")

	FUType_FP_ADD = FUType("FP_ADD")
	FUType_FP_MULT_DIV = FUType("FP_MULT_DIV")

	FUType_MEM_PORT = FUType("MEM_PORT")
)

var FUTypes = []FUType{
	FUType_INT_ALU,
	FUType_INT_MULT_DIV,

	FUType_FP_ADD,
	FUType_FP_MULT_DIV,

	FUType_MEM_PORT,
}

type FUOperationType string

const (
	FUOperationType_NONE = FUOperationType("NONE")

	FUOperationType_INT_ALU = FUOperationType("INT_ALU")
	FUOperationType_INT_MULT = FUOperationType("INT_MULT")
	FUOperationType_INT_DIV = FUOperationType("INT_DIV")

	FUOperationType_FP_ADD = FUOperationType("FP_ADD")
	FUOperationType_FP_CMP = FUOperationType("FP_CMP")
	FUOperationType_FP_CVT = FUOperationType("FP_CVT")
	FUOperationType_FP_MULT = FUOperationType("FP_MULT")
	FUOperationType_FP_DIV = FUOperationType("FP_DIV")
	FUOperationType_FP_SQRT = FUOperationType("FP_SQRT")

	FUOperationType_READ_PORT = FUOperationType("READ_PORT")
	FUOperationType_WRITE_PORT = FUOperationType("WRITE_PORT")
)

var FUOperationTypes = []FUOperationType{
	FUOperationType_NONE,

	FUOperationType_INT_ALU,
	FUOperationType_INT_MULT,
	FUOperationType_INT_DIV,

	FUOperationType_FP_ADD,
	FUOperationType_FP_CMP,
	FUOperationType_FP_CVT,
	FUOperationType_FP_MULT,
	FUOperationType_FP_DIV,
	FUOperationType_FP_SQRT,

	FUOperationType_READ_PORT,
	FUOperationType_WRITE_PORT,
}

type FUOperation struct {
	OperationLatency uint32
	IssueLatency     uint32
}

func NewFUOperation(operationLatency uint32, issueLatency uint32) *FUOperation {
	var fuOperation = &FUOperation{
		OperationLatency:operationLatency,
		IssueLatency:issueLatency,
	}

	return fuOperation
}

type FUDescriptor struct {
	FUPool     *FUPool
	FUType     FUType
	Quantity   uint32
	NumFree    uint32
	Operations map[FUOperationType]*FUOperation
}

func NewFUDescriptor(fuPool *FUPool, fuType FUType, quantity uint32) *FUDescriptor {
	var fuDescriptor = &FUDescriptor{
		FUPool:fuPool,
		FUType:fuType,
		Quantity:quantity,
		NumFree:quantity,
		Operations:make(map[FUOperationType]*FUOperation),
	}

	return fuDescriptor
}

func (fuDescriptor *FUDescriptor) AddFUOperation(fuOperationType FUOperationType, operationLatency uint32, issueLatency uint32) *FUDescriptor {
	fuDescriptor.Operations[fuOperationType] = NewFUOperation(operationLatency, issueLatency)
	fuDescriptor.FUPool.FUOperationToFUTypes[fuOperationType] = fuDescriptor.FUType
	return fuDescriptor
}

func (fuDescriptor *FUDescriptor) ReleaseAll() {
	fuDescriptor.NumFree = fuDescriptor.Quantity
}

func (fuDescriptor *FUDescriptor) Full() bool {
	return fuDescriptor.NumFree == 0
}

type FUPool struct {
	Core                               Core

	Descriptors                        map[FUType]*FUDescriptor

	FUOperationToFUTypes               map[FUOperationType]FUType

	NumStallsOnNoFreeFU                map[FUType]int32
	NumStallsOnAcquireFailedOnNoFreeFU map[FUOperationType]int32
}

func NewFUPool(core Core) *FUPool {
	var fuPool = &FUPool{
		Core:core,

		Descriptors:make(map[FUType]*FUDescriptor),
		FUOperationToFUTypes: make(map[FUOperationType]FUType),
		NumStallsOnNoFreeFU: make(map[FUType]int32),
		NumStallsOnAcquireFailedOnNoFreeFU: make(map[FUOperationType]int32),
	}

	fuPool.AddFUDescriptor(
		FUType_INT_ALU, 8,
	).AddFUOperation(
		FUOperationType_INT_ALU, 2, 1,
	)

	fuPool.AddFUDescriptor(
		FUType_INT_MULT_DIV, 2,
	).AddFUOperation(
		FUOperationType_INT_MULT, 3, 1,
	).AddFUOperation(
		FUOperationType_INT_DIV, 20, 19,
	)

	fuPool.AddFUDescriptor(
		FUType_FP_ADD, 8,
	).AddFUOperation(
		FUOperationType_FP_ADD, 4, 1,
	).AddFUOperation(
		FUOperationType_FP_CMP, 4, 1,
	).AddFUOperation(
		FUOperationType_FP_CVT, 4, 1,
	)

	fuPool.AddFUDescriptor(
		FUType_FP_MULT_DIV, 2,
	).AddFUOperation(
		FUOperationType_FP_MULT, 8, 1,
	).AddFUOperation(
		FUOperationType_FP_DIV, 40, 20,
	).AddFUOperation(
		FUOperationType_FP_SQRT, 80, 40,
	)

	fuPool.AddFUDescriptor(
		FUType_MEM_PORT, 4,
	).AddFUOperation(
		FUOperationType_READ_PORT, 1, 1,
	).AddFUOperation(
		FUOperationType_WRITE_PORT, 1, 1,
	)

	for _, fuType := range FUTypes {
		fuPool.NumStallsOnNoFreeFU[fuType] = 0
	}

	for _, fuOperationType := range FUOperationTypes {
		fuPool.NumStallsOnAcquireFailedOnNoFreeFU[fuOperationType] = 0
	}

	return fuPool
}

func (fuPool *FUPool) AddFUDescriptor(fuType FUType, quantity uint32) *FUDescriptor {
	var descriptor = NewFUDescriptor(fuPool, fuType, quantity)
	fuPool.Descriptors[fuType] = descriptor
	return descriptor
}