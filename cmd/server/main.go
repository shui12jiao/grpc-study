package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"pcbook/pb"
	"pcbook/service"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

const (
	secretKey        = "secret"
	tokenDuration    = time.Minute * 10
	serverCertFile   = "cert/server-cert.pem"
	serverCertKey    = "cert/server-key.pem"
	clientCACertFile = "cert/ca-cert.pem"
)

func main() {
	port := flag.Int("port", 0, "the server port")
	enableTLS := flag.Bool("tls", false, "enable SSL/TLS")
	serverType := flag.String("type", "grpc", "server type, grpc or rest")
	endPoint := flag.String("endpoint", "", "grpc endpoint")
	flag.Parse()

	jwtManager := service.NewJWTManager(secretKey, tokenDuration)
	userStore := service.NewInMemoryUserStore()
	authServer := service.NewAuthServer(userStore, jwtManager)
	err := seedUsers(userStore)
	if err != nil {
		log.Fatal("failed to seed users: ", err)
	}

	laptopServer := service.NewLaptopServer(service.NewInMemoryLaptopStore(), service.NewDiskImageStore("img"), service.NewInMemoryRatingStore())

	address := fmt.Sprintf("0.0.0.0:%d", *port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatal("failed to listen: ", err)
	}

	if *serverType == "grpc" {
		err = runGRPCServer(authServer, laptopServer, jwtManager, *enableTLS, listener)
	} else if *serverType == "rest" {
		err = runRESTServer(authServer, laptopServer, jwtManager, *enableTLS, listener, *endPoint)
	} else {
		log.Fatal("unknown server type: ", *serverType)
	}
	if err != nil {
		log.Fatal("failed to start server: ", err)
	}
}

func runGRPCServer(authServer pb.AuthServiceServer, laptopServer pb.LaptopServiceServer, jwtManager *service.JWTMaganer, enableTLS bool, listener net.Listener) error {
	interceptor := service.NewAuthInterceptor(jwtManager, accessibleRoles())
	serverOption := []grpc.ServerOption{
		grpc.UnaryInterceptor(interceptor.Unary()),
		grpc.StreamInterceptor(interceptor.Stream()),
	}

	if enableTLS {
		tlsCredentials, err := loadTLSCredentials()
		if err != nil {
			return fmt.Errorf("failed to load tls credentials: %w", err)
		}
		serverOption = append(serverOption, grpc.Creds(tlsCredentials))
	}

	grpcServer := grpc.NewServer(serverOption...)

	pb.RegisterAuthServiceServer(grpcServer, authServer)
	pb.RegisterLaptopServiceServer(grpcServer, laptopServer)
	reflection.Register(grpcServer)

	log.Printf("start GRPC server at: %s, TLS=%t", listener.Addr().String(), enableTLS)
	return grpcServer.Serve(listener)
}

func runRESTServer(authServer pb.AuthServiceServer,
	laptopServer pb.LaptopServiceServer,
	jwtManager *service.JWTMaganer,
	enableTLS bool,
	listener net.Listener,
	grpcEndpoint string) error {
	mux := runtime.NewServeMux()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dailOptions := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}

	err := pb.RegisterAuthServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, dailOptions)
	if err != nil {
		return err
	}
	err = pb.RegisterLaptopServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, dailOptions)
	if err != nil {
		return err
	}

	log.Printf("start REST server at: %s, TLS=%t", listener.Addr().String(), enableTLS)
	// Start HTTP server (and proxy calls to gRPC server endpoint)
	if enableTLS {
		return http.ServeTLS(listener, mux, serverCertFile, serverCertKey)
	}
	return http.Serve(listener, mux)

}

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	//Load server's certificate and private key
	serverCert, err := tls.LoadX509KeyPair(serverCertFile, serverCertKey)
	if err != nil {
		return nil, err
	}

	//Load certificate of the CA who signed the client's certificate
	pemClientCA, err := ioutil.ReadFile(clientCACertFile)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemClientCA) {
		return nil, fmt.Errorf("failed to append ca certs")
	}

	config := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certPool,
	}
	return credentials.NewTLS(config), nil
}

func accessibleRoles() map[string][]string {
	const laptopServerPath = "/pcbook.LaptopService/"
	return map[string][]string{
		laptopServerPath + "CreateLaptop": {"admin"},
		laptopServerPath + "UplaodImage":  {"admin"},
		laptopServerPath + "RateLaptop":   {"admin", "user"},
		// laptopServerPath + "SearchLaptop":   {any},
	}
}

func seedUsers(userStore service.UserStore) error {
	err := createUser(userStore, "admin1", "secret_admin", "admin")
	if err != nil {
		return err
	}
	return createUser(userStore, "user1", "secret_user", "user")
}

func createUser(userStore service.UserStore, username, password, role string) error {
	user, err := service.NewUser(username, password, role)
	if err != nil {
		return err
	}
	return userStore.Save(user)
}
