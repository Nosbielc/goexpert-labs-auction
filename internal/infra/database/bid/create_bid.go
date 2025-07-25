package bid

import (
	"context"
	"fullcycle-auction_go/configuration/logger"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/entity/bid_entity"
	"fullcycle-auction_go/internal/infra/database/auction"
	"fullcycle-auction_go/internal/internal_error"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
)

type BidEntityMongo struct {
	Id        string  `bson:"_id"`
	UserId    string  `bson:"user_id"`
	AuctionId string  `bson:"auction_id"`
	Amount    float64 `bson:"amount"`
	Timestamp int64   `bson:"timestamp"`
}

type BidRepository struct {
	Collection          *mongo.Collection
	AuctionRepository   *auction.AuctionRepository
	auctionMaxDuration  time.Duration
	auctionEndTimeMap   map[string]time.Time
	auctionEndTimeMutex *sync.Mutex
}

func NewBidRepository(database *mongo.Database, auctionRepository *auction.AuctionRepository) *BidRepository {
	return &BidRepository{
		auctionMaxDuration:  getAuctionMaxDuration(),
		auctionEndTimeMap:   make(map[string]time.Time),
		auctionEndTimeMutex: &sync.Mutex{},
		Collection:          database.Collection("bids"),
		AuctionRepository:   auctionRepository,
	}
}

func (bd *BidRepository) CreateBid(
	ctx context.Context,
	bidEntities []bid_entity.Bid) *internal_error.InternalError {
	var wg sync.WaitGroup
	for _, bid := range bidEntities {
		wg.Add(1)
		go func(bidValue bid_entity.Bid) {
			defer wg.Done()

			bd.AuctionRepository.AuctionStatusMapMutex.Lock()
			auctionStatus, okStatus := bd.AuctionRepository.AuctionStatusMap[bidValue.AuctionId]
			bd.AuctionRepository.AuctionStatusMapMutex.Unlock()

			bd.auctionEndTimeMutex.Lock()
			auctionEndTime, okEndTime := bd.auctionEndTimeMap[bidValue.AuctionId]
			bd.auctionEndTimeMutex.Unlock()

			bidEntityMongo := &BidEntityMongo{
				Id:        bidValue.Id,
				UserId:    bidValue.UserId,
				AuctionId: bidValue.AuctionId,
				Amount:    bidValue.Amount,
				Timestamp: bidValue.Timestamp.Unix(),
			}

			if okEndTime && okStatus {
				now := time.Now()
				if auctionStatus == auction_entity.Completed || now.After(auctionEndTime) {
					return
				}

				if _, err := bd.Collection.InsertOne(ctx, bidEntityMongo); err != nil {
					logger.Error("Error trying to insert bid", err)
					return
				}

				return
			}

			auctionEntity, err := bd.AuctionRepository.FindAuctionById(ctx, bidValue.AuctionId)
			if err != nil {
				logger.Error("Error trying to find auction by id", err)
				return
			}
			if auctionEntity.Status == auction_entity.Completed {
				return
			}

			bd.AuctionRepository.AuctionStatusMapMutex.Lock()
			bd.AuctionRepository.AuctionStatusMap[bidValue.AuctionId] = auctionEntity.Status
			bd.AuctionRepository.AuctionStatusMapMutex.Unlock()

			bd.auctionEndTimeMutex.Lock()
			bd.auctionEndTimeMap[bidValue.AuctionId] = auctionEntity.Timestamp.Add(bd.auctionMaxDuration)
			bd.auctionEndTimeMutex.Unlock()

			if _, err := bd.Collection.InsertOne(ctx, bidEntityMongo); err != nil {
				logger.Error("Error trying to insert bid", err)
				return
			}
		}(bid)
	}
	wg.Wait()
	return nil
}

func getAuctionMaxDuration() time.Duration {
	auctionMaxDuration := os.Getenv("AUCTION_MAX_DURATION")
	duration, err := time.ParseDuration(auctionMaxDuration)
	if err != nil {
		return 5 * time.Minute
	}

	return duration
}
