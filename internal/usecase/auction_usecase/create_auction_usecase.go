package auction_usecase

import (
	"context"
	"fullcycle-auction_go/configuration/logger"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/entity/bid_entity"
	"fullcycle-auction_go/internal/internal_error"
	"fullcycle-auction_go/internal/usecase/bid_usecase"
	"os"
	"time"
)

type AuctionInputDTO struct {
	ProductName string           `json:"product_name" binding:"required,min=1"`
	Category    string           `json:"category" binding:"required,min=2"`
	Description string           `json:"description" binding:"required,min=10,max=200"`
	Condition   ProductCondition `json:"condition" binding:"oneof=1 2 3"`
}

type AuctionOutputDTO struct {
	Id          string           `json:"id"`
	ProductName string           `json:"product_name"`
	Category    string           `json:"category"`
	Description string           `json:"description"`
	Condition   ProductCondition `json:"condition"`
	Status      AuctionStatus    `json:"status"`
	Timestamp   time.Time        `json:"timestamp" time_format:"2006-01-02 15:04:05"`
}

type WinningInfoOutputDTO struct {
	Auction AuctionOutputDTO          `json:"auction"`
	Bid     *bid_usecase.BidOutputDTO `json:"bid,omitempty"`
}

func NewAuctionUseCase(
	auctionRepositoryInterface auction_entity.AuctionRepositoryInterface,
	bidRepositoryInterface bid_entity.BidEntityRepository) AuctionUseCaseInterface {

	auctionCloseInterval := getAuctionCloseInterval()
	auctionMaxDuration := getAuctionMaxDuration()

	auctionUseCase := &AuctionUseCase{
		auctionRepositoryInterface: auctionRepositoryInterface,
		bidRepositoryInterface:     bidRepositoryInterface,
		auctionCloseInterval:       auctionCloseInterval,
		auctionMaxDuration:         auctionMaxDuration,
		timer:                      time.NewTimer(auctionCloseInterval),
	}

	auctionUseCase.triggerCloseRoutine(context.Background())

	return auctionUseCase
}

type AuctionUseCaseInterface interface {
	CreateAuction(
		ctx context.Context,
		auctionInput AuctionInputDTO) (*AuctionOutputDTO, *internal_error.InternalError)

	FindAuctionById(
		ctx context.Context, id string) (*AuctionOutputDTO, *internal_error.InternalError)

	FindAuctions(
		ctx context.Context,
		status AuctionStatus,
		category, productName string) ([]AuctionOutputDTO, *internal_error.InternalError)

	FindWinningBidByAuctionId(
		ctx context.Context,
		auctionId string) (*WinningInfoOutputDTO, *internal_error.InternalError)
}

type ProductCondition int64
type AuctionStatus int64

type AuctionUseCase struct {
	auctionRepositoryInterface auction_entity.AuctionRepositoryInterface
	bidRepositoryInterface     bid_entity.BidEntityRepository

	timer                *time.Timer
	auctionCloseInterval time.Duration
	auctionMaxDuration   time.Duration
}

func (au *AuctionUseCase) triggerCloseRoutine(ctx context.Context) {
	go func() {
		for range au.timer.C {
			auctions, err := au.auctionRepositoryInterface.FindAuctions(ctx, auction_entity.Active, "", "")
			if err != nil {
				logger.Error("error trying to automatically close auctions", err)
			}

			for _, auction := range auctions {
				if time.Now().After(auction.Timestamp.Add(au.auctionMaxDuration)) {
					if err := au.auctionRepositoryInterface.CloseAuction(ctx, auction.Id); err != nil {
						logger.Error("error trying to close auction", err)
					}
				}
			}

			au.timer.Reset(au.auctionCloseInterval)
		}
	}()
}

func (au *AuctionUseCase) CreateAuction(
	ctx context.Context,
	auctionInput AuctionInputDTO) (*AuctionOutputDTO, *internal_error.InternalError) {
	auction, err := auction_entity.CreateAuction(
		auctionInput.ProductName,
		auctionInput.Category,
		auctionInput.Description,
		auction_entity.ProductCondition(auctionInput.Condition))
	if err != nil {
		return nil, err
	}

	if err := au.auctionRepositoryInterface.CreateAuction(
		ctx, auction); err != nil {
		return nil, err
	}

	return &AuctionOutputDTO{
		Id:          auction.Id,
		ProductName: auction.ProductName,
		Category:    auction.Category,
		Description: auction.Description,
		Condition:   ProductCondition(auction.Condition),
		Status:      AuctionStatus(auction.Status),
		Timestamp:   auction.Timestamp,
	}, nil
}

func getAuctionCloseInterval() time.Duration {
	auctionCloseInterval := os.Getenv("AUCTION_CLOSE_INTERVAL")
	duration, err := time.ParseDuration(auctionCloseInterval)
	if err != nil {
		return 1 * time.Minute
	}

	return duration
}

func getAuctionMaxDuration() time.Duration {
	auctionMaxDuration := os.Getenv("AUCTION_MAX_DURATION")
	duration, err := time.ParseDuration(auctionMaxDuration)
	if err != nil {
		return 5 * time.Minute
	}

	return duration
}
