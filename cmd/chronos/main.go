package main

import (
	"flag"
	"fmt"
	"github.com/pdufour/Chronos/domain"
	"github.com/pdufour/Chronos/output"
	"github.com/pdufour/Chronos/pointerAnalysis"
	"github.com/pdufour/Chronos/ssaUtils"
	"github.com/pdufour/Chronos/utils"
	"golang.org/x/tools/go/ssa"
	"os"
)

func main() {
	defaultFile := flag.String("file", "", "The file containing the entry point of the program")
	defaultModulePath := flag.String("mod", "", "PPath to the module where the search should be performed. Path to module can be relative or absolute but must contain the format:{VCS}/{organization}/{package}. Packages outside this path are excluded rom the search.")
	flag.Parse()
	if *defaultFile == "" {
		fmt.Printf("Please provide a file to load\n")
		os.Exit(1)
	}
	if *defaultModulePath == "" {
		fmt.Printf("Please provide a path to the module. path to module can be relative or absolute but must contain the format:{VCS}/{organization}/{package}.\n")
		os.Exit(1)
	}
	domain.GoroutineCounter = utils.NewCounter()
	domain.GuardedAccessCounter = utils.NewCounter()
	domain.PosIDCounter = utils.NewCounter()

	ssaProg, ssaPkg, err := ssaUtils.LoadPackage(*defaultFile, *defaultModulePath)
	if err != nil {
		fmt.Printf("Failed loading with the following error:%s\n", err)
		os.Exit(1)
	}
	entryFunc := ssaPkg.Func("main")
	err = ssaUtils.InitPreProcess(ssaProg, *defaultModulePath)
	if err != nil {
		fmt.Print(err)
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
