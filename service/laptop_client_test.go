package service_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"pcbook/pb"
	"pcbook/sample"
	"pcbook/serializer"
	"pcbook/service"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestClientCreateLaptop(t *testing.T) {
	t.Parallel()

	laptopStore := service.NewInMemoryLaptopStore()
	_, serverAddress := startTestLaptopServer(t, laptopStore, nil, nil)
	laptopClient := newTestLaptopClient(t, serverAddress)

	laptop := sample.NewLaptop()
	expectedID := laptop.Id
	req := &pb.CreateLaptopRequest{
		Laptop: laptop,
	}

	res, err := laptopClient.CreateLaptop(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, expectedID, res.Id)

	other, err := laptopStore.Find(res.Id)
	require.NoError(t, err)
	require.NotNil(t, other)
	requireSameLaptop(t, laptop, other)
}

func TestClientSearchLaptop(t *testing.T) {
	t.Parallel()

	filter := pb.Filter{
		MaxPriceUsd: 2500,
		MinCpuCores: 4,
		MinCpuGhz:   2.5,
		MinRam:      &pb.Memory{Value: 8, Unit: pb.Memory_GIGABYTE},
	}

	store := service.NewInMemoryLaptopStore()
	exprctedIds := make(map[string]bool)

	for i := 0; i < 6; i++ {
		laptop := sample.NewLaptop()
		switch i {
		case 0: //false
			laptop.PriceUsd = 3000
		case 1: //false
			laptop.Cpu.NumberCores = 2
		case 2: //false
			laptop.Cpu.MinGhz = 1.5
		case 3: //false
			laptop.Ram = &pb.Memory{Value: 4096, Unit: pb.Memory_MEGABYTE}
		case 4: //true
			laptop.PriceUsd = 2500
			laptop.Cpu.NumberCores = 4
			laptop.Cpu.MinGhz = 2.5
			laptop.Ram = &pb.Memory{Value: 8, Unit: pb.Memory_GIGABYTE}
			exprctedIds[laptop.Id] = true
		case 5: //true
			laptop.PriceUsd = 2100
			laptop.Cpu.NumberCores = 8
			laptop.Cpu.MinGhz = 3.5
			laptop.Ram = &pb.Memory{Value: 32, Unit: pb.Memory_GIGABYTE}
			exprctedIds[laptop.Id] = true
		}
		err := store.Save(laptop)
		require.NoError(t, err)
	}
	_, serverAddress := startTestLaptopServer(t, store, nil, nil)
	laptopClient := newTestLaptopClient(t, serverAddress)
	req := &pb.SearchLaptopRequest{
		Filter: &filter,
	}
	stream, err := laptopClient.SearchLaptop(context.Background(), req)
	require.NoError(t, err)

	found := 0
	for {
		res, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		require.Contains(t, exprctedIds, res.GetLaptop().GetId())
		found++
	}
	require.Equal(t, len(exprctedIds), found)
}

func TestClientUploadImage(t *testing.T) {
	t.Parallel()

	testImageFolder := "../tmp"

	laptopStore := service.NewInMemoryLaptopStore()
	imageStore := service.NewDiskImageStore(testImageFolder)

	_, serverAddress := startTestLaptopServer(t, laptopStore, imageStore, nil)
	laptopClient := newTestLaptopClient(t, serverAddress)

	imagePath := fmt.Sprintf("%s/%s", testImageFolder, "laptop.jpg")
	file, err := os.Open(imagePath)
	require.NoError(t, err)
	defer file.Close()

	laptop := sample.NewLaptop()
	err = laptopStore.Save(laptop)
	require.NoError(t, err)

	laptopId := laptop.GetId()
	imageType := filepath.Ext(imagePath)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stream, err := laptopClient.UploadImage(ctx)
	require.NoError(t, err)

	req := &pb.UploadImageRequest{
		Data: &pb.UploadImageRequest_Info{Info: &pb.ImageInfo{LaptopId: laptopId, ImageType: imageType}},
	}
	err = stream.Send(req)
	require.NoError(t, err)

	reader := bufio.NewReader(file)
	buffer := make([]byte, 1024)
	size := 0
	for {
		n, err := reader.Read(buffer)
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		size += n

		err = stream.Send(&pb.UploadImageRequest{Data: &pb.UploadImageRequest_ChunkData{ChunkData: buffer[:n]}})
		require.NoError(t, err)
	}
	res, err := stream.CloseAndRecv()
	require.NoError(t, err)
	require.EqualValues(t, size, res.GetSize())
	require.NotZero(t, res.GetId())

	savedImagePath := fmt.Sprintf("%s/%s%s", testImageFolder, res.GetId(), imageType)
	require.FileExists(t, savedImagePath)
	time.Sleep(time.Millisecond * 200)
	require.NoError(t, os.Remove(savedImagePath))
}

func TestClientRateLaptop(t *testing.T) {
	t.Parallel()

	laptopStore := service.NewInMemoryLaptopStore()
	ratingStore := service.NewInMemoryRatingStore()

	_, serverAddress := startTestLaptopServer(t, laptopStore, nil, ratingStore)
	laptopClient := newTestLaptopClient(t, serverAddress)

	n := 4
	laptopIds := make([]string, n)
	ratings := make([][3]float64, n)
	averages := make([]float64, n)

	for i := 0; i < n; i++ {
		laptop := sample.NewLaptop()
		laptopIds[i] = laptop.GetId()
		err := laptopStore.Save(laptop)
		require.NoError(t, err)

		ratings[i][0] = sample.NewRating()
		ratings[i][1] = sample.NewRating()
		ratings[i][2] = sample.NewRating()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Second)
	defer cancel()

	//send 3 times
	for j := 0; j < 3; j++ {
		//
		stream, err := laptopClient.RateLaptop(ctx)
		require.NoError(t, err)

		//send request
		for i, laptopId := range laptopIds {
			req := &pb.RateLaptopRequest{
				LaptopId: laptopId,
				Rating:   ratings[i][j],
			}
			err := stream.Send(req)
			require.NoError(t, err)
		}

		//close stream
		err = stream.CloseSend()
		require.NoError(t, err)

		for i := 0; ; i++ {
			res, err := stream.Recv()
			if err == io.EOF {
				require.Equal(t, i, n)
				break
			}
			require.NoError(t, err)
			require.Equal(t, laptopIds[i], res.GetLaptopId())
			require.Equal(t, uint32(j+1), res.GetRatedCount())
			averages[i] = (averages[i]*float64(j) + ratings[i][j]) / float64(j+1)
			require.Equal(t, averages[i], res.GetAverageRating())
		}
	}
}

func startTestLaptopServer(t *testing.T, laptopStore service.LaptopStore, imageStore service.ImageStore, ratingStore service.RatingStore) (*service.LaptopServer, string) {
	laptopServer := service.NewLaptopServer(laptopStore, imageStore, ratingStore)
	grpcServer := grpc.NewServer()

	pb.RegisterLaptopServiceServer(grpcServer, laptopServer)

	listener, err := net.Listen("tcp", ":0") //random avaliable port
	require.NoError(t, err)
	go grpcServer.Serve(listener)

	return laptopServer, listener.Addr().String()
}

func newTestLaptopClient(t *testing.T, serverAddress string) pb.LaptopServiceClient {
	conn, err := grpc.Dial(serverAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return pb.NewLaptopServiceClient(conn)
}

func requireSameLaptop(t *testing.T, laptop1, laptop2 *pb.Laptop) {
	json1, err := serializer.ProtobufToJSON(laptop1)
	require.NoError(t, err)

	json2, err := serializer.ProtobufToJSON(laptop2)
	require.NoError(t, err)

	require.Equal(t, json1, json2)
}
