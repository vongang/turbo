package turbo

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"text/template"
)

func CreateProject(pkgPath, serviceName string) {
	BeforeLoadConfig(pkgPath)
	createFolders()
	createFiles(serviceName)
	LoadServiceConfig()
	GenerateSwitcher()
	GenerateProtobufStub()
	generateServiceMain()
	generateServiceImpl()
	generateHTTPMain()
}

func createFolders() {
	os.Mkdir(serviceRootPath, 0755)
	os.Mkdir(serviceRootPath+"/gen", 0755)
	os.Mkdir(serviceRootPath+"/service", 0755)
	os.Mkdir(serviceRootPath+"/service/impl", 0755)
}

func createFiles(serviceName string) {
	createServiceYaml(serviceName)
	createProto(serviceName)
}

func createServiceYaml(serviceName string) {
	tmpl, err := template.New("yaml").Parse(serviceYaml)
	if err != nil {
		panic(err)
	}
	f, _ := os.Create(serviceRootPath + "/service.yaml")
	err = tmpl.Execute(f, serviceYamlValues{ServiceName: serviceName})
	if err != nil {
		panic(err)
	}
}

type serviceYamlValues struct {
	ServiceName string
}

var serviceYaml string = `config:
  port: 8081
  service_name: {{.ServiceName}}
  service_address: 127.0.0.1:50051

urlmapping:
  - GET /hello SayHello
`

func createProto(serviceName string) {
	nameLower := strings.ToLower(serviceName)
	tmpl, err := template.New("proto").Parse(proto)
	if err != nil {
		panic(err)
	}
	f, _ := os.Create(serviceRootPath + "/" + nameLower + ".proto")
	err = tmpl.Execute(f, protoValues{ServiceName: serviceName})
	if err != nil {
		panic(err)
	}
}

type protoValues struct {
	ServiceName string
}

var proto string = `syntax = "proto3";
package gen;

message SayHelloRequest {
    string yourName = 1;
}

message SayHelloResponse {
    string message = 1;
}

service {{.ServiceName}} {
    rpc sayHello (SayHelloRequest) returns (SayHelloResponse) {}
}
`

/*
generate switcher.go, [service_name].pb.go, service/[service_name].go, " +
		"service/impl/[service_name]impl.go
*/
func GenerateSwitcher() {
	var casesStr string
	for _, v := range UrlServiceMap {
		tmpl, err := template.New("cases").Parse(cases)
		if err != nil {
			panic(err)
		}
		var casesBuf bytes.Buffer
		err = tmpl.Execute(&casesBuf, method{configs[SERVICE_NAME], v[2]})
		casesStr = casesStr + casesBuf.String()
	}
	tmpl, err := template.New("switcher").Parse(switcherFunc)
	if err != nil {
		panic(err)
	}
	if _, err := os.Stat(serviceRootPath + "/gen"); os.IsNotExist(err) {
		os.Mkdir(serviceRootPath+"/gen", 0755)
	}
	f, _ := os.Create(serviceRootPath + "/gen/switcher.go")
	err = tmpl.Execute(f, handlerContent{Cases: casesStr})
	if err != nil {
		panic(err)
	}
}

type method struct {
	ServiceName string
	MethodName  string
}

type handlerContent struct {
	Cases string
}

var switcherFunc string = `package gen

import (
	"reflect"
	"net/http"
	"turbo"
	"fmt"
)

/*
this is a generated file, DO NOT EDIT!
 */
var Switcher = func(methodName string, resp http.ResponseWriter, req *http.Request) {
	switch methodName { {{.Cases}}
	default:
		resp.Write([]byte(fmt.Sprintf("No such grpc method[%s]", methodName)))
	}
}
`

var cases string = `
	case "{{.MethodName}}":
		request := {{.MethodName}}Request{}
		theType := reflect.TypeOf(request)
		theValue := reflect.ValueOf(&request).Elem()
		fieldNum := theType.NumField()
		for i := 0; i < fieldNum; i++ {
			fieldName := theType.Field(i).Name
			v, ok := req.Form[turbo.ToSnakeCase(fieldName)]
			if ok && len(v) > 0 {
				theValue.FieldByName(fieldName).SetString(v[0])
			}
		}
		params := make([]reflect.Value, 2)
		params[0] = reflect.ValueOf(req.Context())
		params[1] = reflect.ValueOf(&request)
		result := reflect.ValueOf(turbo.GrpcService().({{.ServiceName}}Client)).MethodByName(methodName).Call(params)
		rsp := result[0].Interface().(*{{.MethodName}}Response)
		if result[1].Interface() == nil {
			resp.Write([]byte(rsp.String() + "\n"))
		} else {
			resp.Write([]byte(result[1].Interface().(error).Error() + "\n"))
		}`

func GenerateProtobufStub() {
	nameLower := strings.ToLower(configs[SERVICE_NAME])
	cmd := "protoc -I " + serviceRootPath + " " + serviceRootPath + "/" + nameLower + ".proto --go_out=plugins=grpc:" + serviceRootPath + "/gen"
	excuteCmd("bash", "-c", cmd)
}

func excuteCmd(cmd string, args ...string) {
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin
	c.Stderr = os.Stderr
	c.Stdout = os.Stdout
	if err := c.Run(); err != nil {
		panic(err)
	}
}

func generateServiceMain() {
	nameLower := strings.ToLower(configs[SERVICE_NAME])
	tmpl, err := template.New("main").Parse(serviceMain)
	if err != nil {
		panic(err)
	}
	f, _ := os.Create(serviceRootPath + "/service/" + nameLower + ".go")
	err = tmpl.Execute(f, serviceMainValues{PkgPath: servicePkgPath, Port: "50051", ServiceName: configs[SERVICE_NAME]})
	if err != nil {
		panic(err)
	}
}

type serviceMainValues struct {
	PkgPath     string
	Port        string
	ServiceName string
}

var serviceMain string = `package main

import (
	"net"
	"log"
	"google.golang.org/grpc"
	"{{.PkgPath}}/service/impl"
	"{{.PkgPath}}/gen"
	"google.golang.org/grpc/reflection"
)

func main() {
	lis, err := net.Listen("tcp", ":{{.Port}}")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	grpcServer := grpc.NewServer()
	gen.Register{{.ServiceName}}Server(grpcServer, &impl.{{.ServiceName}}{})

	reflection.Register(grpcServer)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
`

func generateServiceImpl() {
	nameLower := strings.ToLower(configs[SERVICE_NAME])
	tmpl, err := template.New("impl").Parse(serviceImpl)
	if err != nil {
		panic(err)
	}
	f, _ := os.Create(serviceRootPath + "/service/impl/" + nameLower + "impl.go")
	err = tmpl.Execute(f, serviceImplValues{PkgPath: servicePkgPath, ServiceName: configs[SERVICE_NAME]})
	if err != nil {
		panic(err)
	}
}

type serviceImplValues struct {
	PkgPath     string
	ServiceName string
}

var serviceImpl string = `package impl

import (
	"golang.org/x/net/context"
	"{{.PkgPath}}/gen"
)

type {{.ServiceName}} struct {
}

func (s *{{.ServiceName}}) SayHello(ctx context.Context, req *gen.SayHelloRequest) (*gen.SayHelloResponse, error) {
	return &gen.SayHelloResponse{Message: "Hello, " + req.YourName}, nil
}
`

func generateHTTPMain() {
	nameLower := strings.ToLower(configs[SERVICE_NAME])
	tmpl, err := template.New("httpmain").Parse(_HTTPMain)
	if err != nil {
		panic(err)
	}
	f, _ := os.Create(serviceRootPath + "/" + nameLower + "api.go")
	err = tmpl.Execute(f, _HTTPMainValues{ServiceName: configs[SERVICE_NAME], PkgPath: servicePkgPath})
	if err != nil {
		panic(err)
	}
}

type _HTTPMainValues struct {
	ServiceName string
	PkgPath     string
}

var _HTTPMain string = `package main

import (
	"turbo"
	"google.golang.org/grpc"
	"{{.PkgPath}}/gen"
)

func main() {
	turbo.StartGrpcHTTPServer("{{.PkgPath}}", grpcClient, gen.Switcher)
}

func grpcClient(conn *grpc.ClientConn) interface{} {
	return gen.New{{.ServiceName}}Client(conn)
}
`
