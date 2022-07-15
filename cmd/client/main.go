package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"pcbook/client"
	"pcbook/pb"
	"pcbook/sample"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	username        = "admin1"
	password        = "secret_admin"
	refreshDuration = time.Minute
)

func main() {
	serverAddress := flag.String("address", "", "the server address")
	enableTLS := flag.Bool("tls", false, "enable SSL/TLS")
	flag.Parse()
	log.Printf("dail server %s, TLS=%t", *serverAddress, *enableTLS)

	transportOption := grpc.WithTransportCredentials(insecure.NewCredentials())

	if *enableTLS {
		tlsCredentials, err := loadTLSCredentials()
		if err != nil {
			log.Fatal("failed to load TLS credentials: ", err)
		}
		transportOption = grpc.WithTransportCredentials(tlsCredentials)
	}

	conn1, err := grpc.Dial(*serverAddress, transportOption)
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	authClient := client.NewAuthClient(conn1, username, password)
	interceptor, err := client.NewAuthInterceptor(authClient, authMethods(), refreshDuration)
	if err != nil {
		log.Fatal("failed to create auth interceptor: ", err)
	}

	conn2, err := grpc.Dial(*serverAddress,
		transportOption,
		grpc.WithUnaryInterceptor(interceptor.Unary()),
		grpc.WithStreamInterceptor(interceptor.Stream()))
	if err != nil {
		log.Fatalf("failed to dial: %v", err)
	}
	laptopClient := client.NewLaptopClient(conn2)
	testRateLaptop(laptopClient, 5)
}

func loadTLSCredentials() (credentials.TransportCredentials, error) {
	//Load certificate of the CA who signed the server's certificate
	pemServerCA, err := ioutil.ReadFile("cert/ca-cert.pem")
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemServerCA) {
		return nil, fmt.Errorf("failed to append ca certs")
	}

	//Load server's certificate and private key
	clientCert, err := tls.LoadX509KeyPair("cert/client-cert.pem", "cert/client-key.pem")
	if err != nil {
		return nil, err
	}

	//Create TLS credentials
	config := &tls.Config{
		RootCAs:      certPool,
		Certificates: []tls.Certificate{clientCert},
	}
	return credentials.NewTLS(config), nil
}

func authMethods() map[string]bool {
	const laptopServerPath = "/pcbook.LaptopService/"

	return map[string]bool{
		laptopServerPath + "CreateLaptop": true,
		laptopServerPath + "UplaodImage":  true,
		laptopServerPath + "RateLaptop":   true,
		// laptopServerPath + "SearchLaptop":   false,
	}
}

func testCreateLaptop(laptopClient *client.LaptopClient) {
	laptopClient.CreateLaptop(sample.NewLaptop())
}

func testSearchLaptop(laptopClient *client.LaptopClient) {
	for i := 0; i < 10; i++ {
		laptopClient.CreateLaptop(sample.NewLaptop())
	}

	filter := &pb.Filter{
		MaxPriceUsd: 2500,
		MinCpuCores: 2,
		MinCpuGhz:   2.5,
		MinRam:      &pb.Memory{Value: 8, Unit: pb.Memory_GIGABYTE},
	}
	laptopClient.SearchLaptop(filter)
}
func testUploadImage(laptopClient *client.LaptopClient) {
	laptop := sample.NewLaptop()
	laptopClient.CreateLaptop(laptop)
	laptopClient.UploadImage(laptop.GetId(), "tmp/laptop.jpg")
}

func testRateLaptop(laptopClient *client.LaptopClient, n int) {
	laptopIds := make([]string, n)
	ratings := make([]float64, n)
	for i := 0; i < n; i++ {
		laptop := sample.NewLaptop()
		laptopClient.CreateLaptop(laptop)
		laptopIds[i] = laptop.GetId()
	}

	for i := 0; i < n; i++ {
		fmt.Print("rate laptop (y/n)?")
		var answer string
		fmt.Scan(&answer)
		if answer == "n" || answer == "N" {
			break
		}

		for i := 0; i < n; i++ {
			ratings[i] = sample.NewRating()
		}

		err := laptopClient.RateLaptop(laptopIds, ratings)
		if err != nil {
			log.Fatal(err)
		}
	}
}
