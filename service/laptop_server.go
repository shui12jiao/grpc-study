package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"pcbook/pb"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//maxsize 1MB
const maxImageSize = 1 << 20

type LaptopServer struct {
	laptopStore LaptopStore
	imageStore  ImageStore
	ratingStore RatingStore
}

func NewLaptopServer(laptopStore LaptopStore, imageStore ImageStore, ratingStore RatingStore) *LaptopServer {
	return &LaptopServer{laptopStore: laptopStore, imageStore: imageStore, ratingStore: ratingStore}
}

func (server *LaptopServer) CreateLaptop(ctx context.Context, req *pb.CreateLaptopRequest) (*pb.CreateLaptopResponse, error) {
	laptop := req.GetLaptop()
	log.Printf("receive a create-laptop request with id: %s", laptop.Id)

	if len(laptop.Id) > 0 {
		//check if the laptop id is valid
		_, err := uuid.Parse(laptop.Id)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "laptop ID is invalid: %v", err)
		}
	} else {
		id, err := uuid.NewRandom()
		if err != nil {
			return nil, status.Errorf(codes.Internal, "cannot generate a new laotop ID: %v", err)
		}
		laptop.Id = id.String()
	}

	//heavy processing
	switch ctx.Err() {
	case context.Canceled:
		log.Print("CreateLaptop request is cancelled")
		return nil, status.Errorf(codes.Canceled, "CreateLaptop request is cancelled")
	case context.DeadlineExceeded:
		log.Print("CreateLaptop request is timed out")
		return nil, status.Errorf(codes.DeadlineExceeded, "CreateLaptop request is timed out")
	default:
	}

	//save the laptop to store
	err := server.laptopStore.Save(laptop)
	if err != nil {
		code := codes.Internal
		if errors.Is(err, ErrAlreadyExists) {
			code = codes.AlreadyExists
		}
		return nil, status.Errorf(code, "cannot save laptop to store: %v", err)
	}

	log.Printf("laptop with id: %s is saved to store", laptop.Id)

	res := &pb.CreateLaptopResponse{
		Id: laptop.Id,
	}

	return res, nil
}

func (server *LaptopServer) SearchLaptop(req *pb.SearchLaptopRequest, stream pb.LaptopService_SearchLaptopServer) error {
	filter := req.GetFilter()
	log.Printf("receive a search-laptop request with filter: %v", filter)

	err := server.laptopStore.Search(stream.Context(), filter, func(laptop *pb.Laptop) error {
		res := &pb.SearchLaptopResponse{Laptop: laptop}

		err := stream.Send(res)
		if err != nil {
			return err
		}

		log.Printf("send laptop with id: %s to client", laptop.GetId())
		return nil
	})
	if err != nil {
		return status.Errorf(codes.Internal, "cannot search laptop from store: %v", err)
	}
	return nil
}

func (server *LaptopServer) UploadImage(stream pb.LaptopService_UploadImageServer) error {
	req, err := stream.Recv()
	if err != nil {
		log.Print(err)
		return status.Error(codes.Unknown, "cannot receive image")
	}

	laptopId := req.GetInfo().GetLaptopId()
	laptopType := req.GetInfo().GetImageType()
	log.Printf("receive a image upload request with laptopId: %s, imageType: %s", laptopId, laptopType)

	laptop, err := server.laptopStore.Find(laptopId)
	if err != nil {
		log.Print(err)
		return status.Errorf(codes.Internal, "cannot find laptop: %v", err)
	}
	if laptop == nil {
		log.Print(err)
		return status.Errorf(codes.InvalidArgument, "laptop with id: %s is not found", laptopId)
	}

	imageData := new(bytes.Buffer)
	imageSize := 0
	for {
		if err := contextError(stream.Context()); err != nil {
			return err
		}

		req, err := stream.Recv()
		if err == io.EOF {
			log.Print("receive all image data")
			break
		} else if err != nil {
			return status.Error(codes.Unknown, "cannot receive image")
		}

		chunk := req.GetChunkData()
		imageSize += len(chunk)
		// log.Printf("receive a image chunk with size: %d", len(chunk)) //debug
		if imageSize > maxImageSize {
			log.Print("image size is too large")
			return status.Error(codes.InvalidArgument, "image size is too large")
		}
		// time.Sleep(time.Second) //debug
		_, err = imageData.Write(chunk)
		if err != nil {
			log.Print(err)
			return status.Error(codes.Internal, "cannot write image data")
		}
	}
	imageId, err := server.imageStore.Save(laptopId, laptopType, imageData)
	if err != nil {
		log.Print(err)
		return status.Errorf(codes.Internal, "cannot save image to store: %v", err)
	}

	res := &pb.UploadImageResponse{
		Id:   imageId,
		Size: uint32(imageSize),
	}

	err = stream.SendAndClose(res)
	if err != nil {
		log.Print(err)
		return status.Errorf(codes.Unknown, "cannot send response to client: %v", err)
	}
	log.Printf("saved image with id: %s, size%d", imageId, imageSize)
	return nil
}

func (server *LaptopServer) RateLaptop(stream pb.LaptopService_RateLaptopServer) error {
	for {
		err := contextError(stream.Context())
		if err != nil {
			return err
		}

		req, err := stream.Recv()
		if err == io.EOF {
			log.Print("receive all ratings")
			break
		} else if err != nil {
			log.Printf("cannot receive rating: %v", err)
			return status.Errorf(codes.Unknown, "cannot receive request: %v", err)
		}

		laptopId := req.GetLaptopId()
		rating := req.GetRating()
		log.Printf("receive a rating request with laptopId: %s, rating: %f", laptopId, rating)

		found, err := server.laptopStore.Find(laptopId)
		if err != nil {
			log.Print(err)
			return status.Errorf(codes.Internal, "cannot find laptop: %v", err)
		}
		if found == nil {
			log.Print(err)
			return status.Errorf(codes.NotFound, "laptop with id: %s is not found", laptopId)
		}

		r, err := server.ratingStore.Add(laptopId, rating)
		if err != nil {
			log.Print(err)
			return status.Errorf(codes.Internal, "cannot add rating: %v", err)
		}
		res := &pb.RateLaptopResponse{
			LaptopId:      laptopId,
			RatedCount:    r.Count,
			AverageRating: r.Sum / float64(r.Count),
		}
		err = stream.Send(res)
		if err != nil {
			log.Print(err)
			return status.Errorf(codes.Unknown, "cannot send response to client: %v", err)
		}
	}
	return nil
}

func contextError(ctx context.Context) error {
	switch ctx.Err() {
	case context.Canceled:
		log.Print("request is cancelled")
		return status.Errorf(codes.Canceled, "request is cancelled")
	case context.DeadlineExceeded:
		log.Print("request is timed out")
		return status.Errorf(codes.DeadlineExceeded, "deadline is exceeded")
	default:
		return nil
	}
}
