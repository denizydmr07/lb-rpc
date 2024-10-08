// generators/stub_generator.go

package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/denizydmr07/zapwrapper/pkg/zapwrapper"
	"go.uber.org/zap"
)

// Service represents a service
// it contains the name of the service and the methods
type Service struct {
	Name    string
	Methods []Method
}

// print the service
func (s Service) String() string {
	str := "Service: " + s.Name + ", "
	for _, method := range s.Methods {
		str += method.String()
	}
	return str
}

// Method represents a method
// it contains the name, params and returns
type Method struct {
	Name    string
	Params  map[string]interface{}
	Returns map[string]interface{}
}

// print the method
func (m Method) String() string {
	str := "Method: " + m.Name + ", "
	str += "Params: "
	for key, value := range m.Params {
		str += key + " " + value.(string) + ", "
	}
	str += "Returns: "
	for key, value := range m.Returns {
		str += key + " " + value.(string) + ", "
	}
	return str
}

// clientStubTemplate is the template for the client stub
// it contains the callRPC function and the method stubs
var clientStubTemplate = `
package stub

import (
	"encoding/json"
	"errors"
	"net"
)

func callRPC(method string, params map[string]interface{}) map[string]interface{} {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	request := map[string]interface{}{
		"method": method,
		"params": params,
	}

	encoder := json.NewEncoder(conn)
	encoder.Encode(request)

	decoder := json.NewDecoder(conn)
	var response map[string]interface{}
	decoder.Decode(&response)

	return response
}

{{range .Methods}}
func {{.Name}}({{range $key, $value := .Params}}{{$key}} {{$value}}, {{end}})( {{range $key, $value := .Returns}}{{$value}}, error {{end}}) {
	var err error
	params := map[string]interface{} {
		{{range $key, $value := .Params}}"{{$key}}": {{$key}},{{end}}
	}
	response := callRPC("{{.Name}}", params)
	// checking if response contains error
	if _, ok := response["error"]; ok {
		err = errors.New(response["error"].(string))
		return -1, err
	}
	return {{range $key, $value := .Returns}}response["{{$key}}"].({{$value}}), err {{end}}
}
{{end}}
`

// addServiceToClient adds the service to the client stub
// it creates a new file under client/stub directory
// and writes the service stub to the file
func addServiceToClient(service Service) {
	// create a new template from the clientStubTemplate variable
	tmpl, err := template.New("clientStub").Parse(clientStubTemplate)
	if err != nil {
		panic(err)
	}

	//create stubs directory under client if it doesn't exist
	os.Mkdir("../client/stub", 0755)

	// create a new file under client/stub directory
	file, err := os.Create("../client/stub/client_stub_" + service.Name + ".go")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// write the service stub to the file
	writer := bufio.NewWriter(file)

	// execute the template
	err = tmpl.Execute(writer, service)
	if err != nil {
		panic(err)
	}

	// flush the buffer
	writer.Flush()
}

var serverStubTemplate = `
package stub

import (
	"encoding/json"
	"time"
	"net"
)

func HandleConnection(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))

	decoder := json.NewDecoder(conn)
	var request map[string]interface{}
	decoder.Decode(&request)

	method := request["method"].(string)
	params := request["params"].(map[string]interface{})

	var response map[string]interface{}

	switch method {
	{{range .Methods}}
	case "{{.Name}}":
		result, err := {{.Name}}({{range $key, $value := .Params}}params["{{$key}}"].({{$value}}), {{end}})

		if err == nil {
			response = map[string]interface{}{
				"result": result,
			}
		} else {
			response = map[string]interface{}{
				"error": err.Error(),
			}
		}
	{{end}}
	default:
		response = map[string]interface{}{
			"error": "Invalid RPC Call Method",
		}
	}

	encoder := json.NewEncoder(conn)
	encoder.Encode(response)
}

// implmentation of Add method
func Add(a float64, b float64) (float64, error) {
	return a + b, nil
}

// implmentation of Sub method
func Sub(a float64, b float64) (float64, error) {
	return a - b, nil
}
`

func addServiceToServer(service Service) {
	fmt.Printf("Service: %s\n", service)
	tmpl, err := template.New("serverStub").Parse(serverStubTemplate)
	if err != nil {
		panic(err)
	}

	os.Mkdir("../server/stub", 0755)

	file, err := os.Create("../server/stub/server_stub_" + service.Name + ".go")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	err = tmpl.Execute(writer, service)
	if err != nil {
		panic(err)
	}

	writer.Flush()
}

func main() {
	// c reating a new logger
	logger := zapwrapper.NewLogger(
		zapwrapper.DefaultFilepath,   // Log file path
		zapwrapper.DefaultMaxBackups, // Max number of log files to retain
		zapwrapper.DefaultLogLevel,   // Log level
	)

	defer logger.Sync() // flushes buffer, if any

	service := &Service{}

	// get the idf file path from the command line
	idfFilePath := "../idl/calculator.idl"
	logger.Debug("idf file path", zap.String("idfFilePath", idfFilePath))

	file, err := os.Open(idfFilePath)
	if err != nil {
		panic(err)
	}

	// read the idf file line by line
	scanner := bufio.NewScanner(file)
	logger.Debug("starting to scan the file")

	// parse the idf file
	for scanner.Scan() {

		line := scanner.Text()

		// if the line contains KEYWORD service, get the service name
		if strings.Contains(line, "service") {
			logger.Debug("Service found", zap.String("line", line))

			service.Name = strings.Fields(line)[1]
		} else if strings.Contains(line, "->") { // if the line contains method, get the method details
			logger.Debug("Method found", zap.String("line", line))

			method := Method{}

			// example: add(int a, int b) -> (int result);
			pattern := `(\w+)\(([^)]*)\)\s*->\s*\(([^)]*)\);` // regex pattern to match the method

			// compile the regex pattern
			re := regexp.MustCompile(pattern)

			matches := re.FindStringSubmatch(line)
			method.Name = matches[1]

			// if method name starts with lowercase, make it uppercase
			if method.Name[0] >= 'a' && method.Name[0] <= 'z' {
				method.Name = strings.Title(method.Name)
			}

			method.Params = make(map[string]interface{})

			// paramsare in the form of "int a, int b, ..."
			params := strings.Split(matches[2], ",")
			for _, param := range params {
				paramParts := strings.Fields(param)
				method.Params[paramParts[1]] = paramParts[0]
			}

			// returns are in the form of "int result, ..."
			method.Returns = make(map[string]interface{})
			returns := strings.Fields(matches[3])
			method.Returns[returns[1]] = returns[0]

			service.Methods = append(service.Methods, method)
		}
	}

	addServiceToClient(*service) // add the service to the client stub
	logger.Debug("Service added to client stub", zap.String("service", service.Name))

	addServiceToServer(*service) // add the service to the server stub
	logger.Debug("Service added to server stub", zap.String("service", service.Name))

	file.Close()
}