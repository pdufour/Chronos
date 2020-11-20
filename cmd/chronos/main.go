package main

import (
	"flag"
	"fmt"
	"github.com/amit-davidson/Chronos/domain"
	"github.com/amit-davidson/Chronos/output"
	"github.com/amit-davidson/Chronos/pointerAnalysis"
	"github.com/amit-davidson/Chronos/ssaUtils"
	"github.com/amit-davidson/Chronos/utils"
	"golang.org/x/tools/go/ssa"
	"os"
)

func main() {
	defaultFile := flag.String("file", "", "The file containing the entry point of the program")
	defaultPkgPath := flag.String("pkg", "", "Path to the to pkg of the file. Tells Chronos where to perform the search. By default, it assumes the file is inside $GOPATH")
	flag.Parse()
	if *defaultFile == "" {
		fmt.Printf("Please provide a file to load\n")
		os.Exit(1)
	}
	domain.GoroutineCounter = utils.NewCounter()
	domain.GuardedAccessCounter = utils.NewCounter()
	domain.PosIDCounter = utils.NewCounter()

	ssaProg, ssaPkg, err := ssaUtils.LoadPackage(*defaultFile)
	if err != nil {
		fmt.Printf("Failed loading with the following error:%s\n", err)
		os.Exit(1)
	}
	entryFunc := ssaPkg.Func("main")
	err = ssaUtils.InitPreProcess(ssaProg, ssaPkg, *defaultPkgPath, entryFunc)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	entryCallCommon := ssa.CallCommon{Value: entryFunc}
	functionState := ssaUtils.HandleCallCommon(domain.NewEmptyContext(), &entryCallCommon, entryFunc.Pos())
	conflictingGAs, err := pointerAnalysis.Analysis(ssaPkg, functionState.GuardedAccesses)
	if err != nil {
		fmt.Printf("Error in analysis:%s\n", err)
		os.Exit(1)
	}
	err = output.GenerateError(conflictingGAs, ssaProg)
	if err != nil {
		fmt.Printf("Error in generating errors:%s\n", err)
		os.Exit(1)
	}
}
