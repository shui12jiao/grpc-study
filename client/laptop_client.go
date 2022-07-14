package client

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"pcbook/pb"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type LaptopClient struct {
	service pb.LaptopServiceClient
}

func NewLaptopClient(cc *grpc.ClientConn) *LaptopClient {
	return &LaptopClient{
		service: pb.NewLaptopServiceClient(cc),
	}
}

func (laptopClient *LaptopClient) CreateLaptop(laptop *pb.Laptop) {
	req := &pb.CreateLaptopRequest{
		Laptop: laptop,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := laptopClient.service.CreateLaptop(ctx, req)
	if err != nil {
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.AlreadyExists {
			log.Print("laptop already exists")
		} else {
			log.Fatalf("failed to create laptop: %v", err)
		}

	}
	log.Printf("created laptop with id: %v", res.Id)
}

func (laptopClient *LaptopClient) SearchLaptop(filter *pb.Filter) {
	req := &pb.SearchLaptopRequest{
		Filter: filter,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stream, err := laptopClient.service.SearchLaptop(ctx, req)
	if err != nil {
		log.Fatalf("failed to search laptop: %v", err)
	}

	for {
		res, err := stream.Recv()
		if err == io.EOF {
			return
		} else if err != nil {
			log.Fatalf("failed to recv: %v", err)
		} else {
			laptop := res.GetLaptop()
			log.Printf("-- found laptop: %v", laptop.GetId())
			log.Printf("  + brand: %v", laptop.GetBrand())
			log.Printf("  + name: %v", laptop.GetName())
			log.Printf("  + price: %v", laptop.GetPriceUsd())
			log.Printf("  + cpu cores: %v", laptop.GetCpu().NumberCores)
			log.Printf("  + cpu min_ghz: %v", laptop.GetCpu().MinGhz)

		}
	}
}

func (laptopClient *LaptopClient) UploadImage(laptopId string, imagePath string) {
	file, err := os.Open(imagePath)
	if err != nil {
		log.Fatalf("failed to open image: %v", err)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := laptopClient.service.UploadImage(ctx)
	if err != nil {
		log.Fatalf("failed to upload image: %v", err)
	}

	req := &pb.UploadImageRequest{
		Data: &pb.UploadImageRequest_Info{
			Info: &pb.ImageInfo{
				LaptopId:  laptopId,
				ImageType: filepath.Ext(imagePath),
			},
		},
	}
	err = stream.Send(req)
	if err != nil {
		log.Fatalf("failed to send request: %v", err)
	}

	reader := bufio.NewReader(file)
	buffer := make([]byte, 1024)
	for {
		n, err := reader.Read(buffer)
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("failed to read: %v", err)
		}

		err = stream.Send(&pb.UploadImageRequest{Data: &pb.UploadImageRequest_ChunkData{ChunkData: buffer[:n]}})
		if err != nil {
			log.Fatalf("failed to send chunk: %v", err)
		}
	}
	res, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("failed to recv response: %v", err)
	}
	fmt.Printf("uploaded image with id: %s and size: %d\n", res.GetId(), res.GetSize())
}

func (laptopClient *LaptopClient) RateLaptop(laptopIds []string, ratings []float64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := laptopClient.service.RateLaptop(ctx)
	if err != nil {
		log.Fatalf("failed to rate laptop: %v", err)
	}
	waitResponse := make(chan error)
	// go routine to receive response
	go func() {
		for {
			res, err := stream.Recv()
			if err == io.EOF {
				log.Print("no more response")
				waitResponse <- nil
				return
			} else if err != nil {
				waitResponse <- fmt.Errorf("failed to recv: %v", err)
			}
			log.Print("received response: ", res)
		}
	}()

	//send request
	for i, laptopId := range laptopIds {
		req := &pb.RateLaptopRequest{
			LaptopId: laptopId,
			Rating:   ratings[i],
		}
		err := stream.Send(req)
		if err != nil {
			return fmt.Errorf("failed to send request: %v", err)
		}

		log.Print("sent request: ", req)

	}
	err = stream.CloseSend()
	if err != nil {
		return fmt.Errorf("cannot close send: %v", err)
	}

	err = <-waitResponse
	return err
}
