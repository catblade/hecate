package hecate

// Hecate is the goddess that helped others find their paths.
// This too will help the http call find its way in cases of
// errors.

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
)

// should probably expand this later to handle a set of errors
// but currently we always return on the first error
type errorInfo struct {
	Trace []string `json:"trace"`
	Err   string   `json:"err"`
}

// Ideally, this can be used to pass stuff back up from errors 
// down the stack if we need to do something above in a function
// that is using this library
type ErrorBox struct {
	Err bool
}

// sometimes you need to dump a stack, and sometimes you don't care.  This also handily
// assembles an ErrorBox for you so you don't have to assemble things *and then*
// pass things in.
func ReportAndPassError(str string, responseWriter http.ResponseWriter, status int, stackDump bool) ErrorBox {
	errorBox := ErrorBox{true}

	HandleError(fmt.Errorf(str), responseWriter, status, stackDump)
	return errorBox
}

// this is the equivalent of passing nil down for the
// builtin type error.
func NoErrorOccurred() ErrorBox {
	errorBox := ErrorBox{false}
	return errorBox
}

var DebugLevel = 0

// This will give you a comprehensive stack, line by line, in json form for easy parsing
func HandleError(err error, responseWriter http.ResponseWriter, status int, stackDump bool) {
	handleAllErrors(err, responseWriter, status, stackDump, false)
}

// This will put the entire stack on one line in he json form, so be careful here.
// This operation can be expensive because underlying code stops the world every
// time you use it.
func HandlePanic(responseWriter http.ResponseWriter) {
	handleAllErrors(fmt.Errorf("Panic."), responseWriter, http.StatusInternalServerError, true, true)
}

// There are places we might not want stacks, so we do have a bool here
func handleAllErrors(err error, responseWriter http.ResponseWriter, status int, stackDump bool, allStacks bool) {

	// we might not want to always output stacks to logs.  They are big and slow and sometimes
	// we just want to return an error code.
	if stackDump == true {
		frames := map[int]*string{}
		if allStacks {
			// one "frame" only.  This one is a full
			// stack dump including goroutines and
			// should only be used in cases of panics,
			// not habitual use.  Ideally we'd pull
			// off each individual frame, but there
			// isn't an easy way to do this due to
			// needing to stopTheWorld to get this version.
			// Yes, I know debug.PrintStack() does this differently
			// but it ends up allocating buffers and doing a bunch
			// of copies, which I don't like.  I'd rather do 
			// a giant allocation once and then shorten it
			// up.
			buf := make([]byte, 1<<20)
			count := runtime.Stack(buf, true)
			temp := string(buf[:count])
			frames[0] = &temp
		} else {
			// take the reporting function off the stack dump
			// and return the rest appropriately
			fpcs := make([]uintptr, 32) // stack max record in runtime lib is 32
			length := runtime.Callers(2, fpcs)
			for i := 0; i < length; i++ {
				f := runtime.FuncForPC(fpcs[i])
				file, line := f.FileLine(fpcs[i])
				temp := fmt.Sprintf("%s:%d %s\n", file, line, f.Name())
				log.Printf("%s", temp)
				frames[i] = &temp
				i++
			}
		}

		// this just tells us if we are outputting the full stack and error to our fancy-pants json call
		// or if we want to fall through (below) to merely returning the error
		if DebugLevel > 0 {
			error := errorInfo{nil, err.Error()}
			for _, frame := range frames {
				error.Trace = append(error.Trace, *frame)
			}

			jsonBytes, err := json.Marshal(error)

			// this isn't ideal, but we can at least return the original error without the stack dump
			if err != nil {
				http.Error(responseWriter, err.Error(), status)
				return
			}

			// all the dumping of the stack
			responseWriter.Header().Set("Content-Type", "application/json")
			responseWriter.WriteHeader(status)
			responseWriter.Write(jsonBytes)
			return
		}
	}

	http.Error(responseWriter, err.Error(), status)
}
