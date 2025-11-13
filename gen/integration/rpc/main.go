//go:build test
// +build test

package main

import (
	"encoding/json"
	"log"
	"net"
	"net/rpc"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Lmsgprefix)
	log.SetPrefix("[defc] ")
}

func main() {
	c, s := net.Pipe()
	go func() {
		srv := rpc.NewServer()
		srv.RegisterName("Arith", NewArithServer(&arith{}))
		srv.ServeCodec(&rpcServerCodec{encoder: json.NewEncoder(s), decoder: json.NewDecoder(s)})
	}()
	cli := NewArithClient(rpc.NewClientWithCodec(&rpcClientCodec{encoder: json.NewEncoder(c), decoder: json.NewDecoder(c)}))
	args := make(chan int, 2)
	args <- 21
	args <- 2
	defer close(args)
	result, err := cli.Multiply(args)
	if err != nil {
		log.Fatalln(err)
	}
	if result != 42 {
		log.Fatalf("unexpected result: %d != 42", result)
	}
	defer func() {
		if recover() == nil {
			log.Fatalln("expects recover, got nil")
		}
	}()
	cli.panic()
	log.Fatalln("expects panic, got nil")
}

//go:generate defc generate -T Arith -o arith.gen.go
type Arith interface {
	Multiply(args chan int) (int, error)
	panic()
}

type arith struct{}

func (impl *arith) Multiply(args chan int) (int, error) {
	reply := 1
	for arg := range args {
		reply *= arg
	}
	return reply, nil
}

func (impl *arith) panic() {}

type rpcServerCodec struct {
	encoder *json.Encoder
	decoder *json.Decoder
}

func (srv *rpcServerCodec) ReadRequestHeader(req *rpc.Request) error {
	var payload Payload
	if err := srv.decoder.Decode(&payload); err != nil {
		log.Fatalln(err)
	}
	req.ServiceMethod = payload.Method
	req.Seq = payload.ID
	return nil
}

func (srv *rpcServerCodec) ReadRequestBody(body any) error {
	var payload Payload
	if err := srv.decoder.Decode(&payload); err != nil {
		log.Fatalln(err)
	}
	switch payload.Method {
	case "Arith.Multiply":
		ch := make(chan int, 2)
		ch <- payload.Args[0]
		ch <- payload.Args[1]
		*body.(*chan int) = ch
		close(ch)
	default:
		log.Fatalf("unknown rpc method %q", payload.Method)
	}
	return nil
}

func (srv *rpcServerCodec) WriteResponse(resp *rpc.Response, body any) error {
	payload := Payload{
		ID:     resp.Seq,
		Method: resp.ServiceMethod,
	}
	if err := srv.encoder.Encode(&payload); err != nil {
		log.Fatalln(err)
	}
	payload.Reply = *body.(*int)
	if err := srv.encoder.Encode(&payload); err != nil {
		log.Fatalln(err)
	}
	return nil
}

func (srv *rpcServerCodec) Close() error {
	return nil
}

type rpcClientCodec struct {
	encoder *json.Encoder
	decoder *json.Decoder
}

func (cli *rpcClientCodec) WriteRequest(req *rpc.Request, body any) error {
	payload := Payload{
		ID:     req.Seq,
		Method: req.ServiceMethod,
	}
	if err := cli.encoder.Encode(&payload); err != nil {
		log.Fatalln(err)
	}
	switch req.ServiceMethod {
	case "Arith.Multiply":
		ch := body.(chan int)
		payload.Args = []int{<-ch, <-ch}
	default:
		log.Fatalf("unknown rpc method %q", req.ServiceMethod)
	}
	if err := cli.encoder.Encode(&payload); err != nil {
		log.Fatalln(err)
	}
	return nil
}

func (cli *rpcClientCodec) ReadResponseHeader(resp *rpc.Response) error {
	var payload Payload
	if err := cli.decoder.Decode(&payload); err != nil {
		log.Fatalln(err)
	}
	resp.ServiceMethod = payload.Method
	resp.Seq = payload.ID
	return nil
}

func (cli *rpcClientCodec) ReadResponseBody(body any) error {
	var payload Payload
	if err := cli.decoder.Decode(&payload); err != nil {
		log.Fatalln(err)
	}
	switch payload.Method {
	case "Arith.Multiply":
		*body.(*int) = payload.Reply
	default:
		log.Fatalf("unknown rpc method %q", payload.Method)
	}
	return nil
}

func (cli *rpcClientCodec) Close() error {
	return nil
}

type Payload struct {
	ID     uint64 `json:"id"`
	Method string `json:"method"`
	Args   []int  `json:"args"`
	Reply  int    `json:"reply"`
}
