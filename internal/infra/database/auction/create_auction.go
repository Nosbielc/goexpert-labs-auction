package auction

import (
	"context"
	"fullcycle-auction_go/configuration/logger"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"fullcycle-auction_go/internal/internal_error"
	"os"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuctionEntityMongo struct {
	Id          string                          `bson:"_id"`
	ProductName string                          `bson:"product_name"`
	Category    string                          `bson:"category"`
	Description string                          `bson:"description"`
	Condition   auction_entity.ProductCondition `bson:"condition"`
	Status      auction_entity.AuctionStatus    `bson:"status"`
	Timestamp   int64                           `bson:"timestamp"`
}

type AuctionRepository struct {
	Collection            *mongo.Collection
	AuctionStatusMap      map[string]auction_entity.AuctionStatus
	AuctionStatusMapMutex *sync.Mutex
	closeOnce             sync.Once
}

func NewAuctionRepository(database *mongo.Database) *AuctionRepository {
	repo := &AuctionRepository{
		Collection:            database.Collection("auctions"),
		AuctionStatusMapMutex: &sync.Mutex{},
		AuctionStatusMap:      make(map[string]auction_entity.AuctionStatus),
	}

	// Inicia a goroutine de fechamento automático apenas uma vez
	repo.closeOnce.Do(func() {
		go repo.startAuctionCloserRoutine()
	})

	return repo
}

func (ar *AuctionRepository) CreateAuction(
	ctx context.Context,
	auctionEntity *auction_entity.Auction) *internal_error.InternalError {
	auctionEntityMongo := &AuctionEntityMongo{
		Id:          auctionEntity.Id,
		ProductName: auctionEntity.ProductName,
		Category:    auctionEntity.Category,
		Description: auctionEntity.Description,
		Condition:   auctionEntity.Condition,
		Status:      auctionEntity.Status,
		Timestamp:   auctionEntity.Timestamp.Unix(),
	}
	_, err := ar.Collection.InsertOne(ctx, auctionEntityMongo)
	if err != nil {
		logger.Error("Error trying to insert auction", err)
		return internal_error.NewInternalServerError("Error trying to insert auction")
	}

	// Agenda o fechamento automático deste leilão específico
	go ar.scheduleAuctionClose(auctionEntity.Id, auctionEntity.Timestamp)

	return nil
}

// CloseAuction fecha um leilão atualizando seu status para Completed
func (ar *AuctionRepository) CloseAuction(
	ctx context.Context,
	auctionId string) *internal_error.InternalError {
	ar.AuctionStatusMapMutex.Lock()
	defer ar.AuctionStatusMapMutex.Unlock()

	filter := bson.M{"_id": auctionId}
	update := bson.M{"$set": bson.M{"status": auction_entity.Completed}}

	_, err := ar.Collection.UpdateOne(ctx, filter, update)
	if err != nil {
		logger.Error("Error trying to close auction", err)
		return internal_error.NewInternalServerError("Error trying to close auction")
	}

	ar.AuctionStatusMap[auctionId] = auction_entity.Completed
	return nil
}

// scheduleAuctionClose agenda o fechamento de um leilão específico
func (ar *AuctionRepository) scheduleAuctionClose(auctionId string, startTime time.Time) {
	auctionInterval := getAuctionInterval()
	closeTime := startTime.Add(auctionInterval)

	// Calcula o tempo de espera até o fechamento
	waitDuration := time.Until(closeTime)

	// Se o leilão já deveria ter fechado, fecha imediatamente
	if waitDuration <= 0 {
		ar.closeAuctionAutomatically(auctionId)
		return
	}

	// Agenda o fechamento
	time.AfterFunc(waitDuration, func() {
		ar.closeAuctionAutomatically(auctionId)
	})
}

// closeAuctionAutomatically fecha um leilão específico automaticamente
func (ar *AuctionRepository) closeAuctionAutomatically(auctionId string) {
	ctx := context.Background()

	// Verifica se o leilão ainda está ativo antes de fechar
	auction, err := ar.FindAuctionById(ctx, auctionId)
	if err != nil {
		logger.Error("Error trying to find auction to close", err)
		return
	}

	if auction.Status == auction_entity.Active {
		if updateErr := ar.CloseAuction(ctx, auctionId); updateErr != nil {
			logger.Error("Error trying to close auction automatically", updateErr)
		} else {
			logger.Info("Auction closed automatically: " + auctionId)
		}
	}
}

// startAuctionCloserRoutine inicia uma goroutine que verifica periodicamente leilões vencidos
func (ar *AuctionRepository) startAuctionCloserRoutine() {
	// Usar um intervalo mais frequente para leilões de curta duração
	auctionInterval := getAuctionInterval()

	// Se o intervalo do leilão é muito curto (< 1 minuto), verificar mais frequentemente
	checkInterval := time.Minute
	if auctionInterval < time.Minute {
		checkInterval = auctionInterval / 2 // Verifica a cada metade do tempo do leilão
		if checkInterval < 10*time.Second {
			checkInterval = 10 * time.Second // Mínimo de 10 segundos
		}
	}

	logger.Info("Starting auction closer routine with interval: " + checkInterval.String())

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		ar.closeExpiredAuctions()
	}
}

// closeExpiredAuctions fecha todos os leilões que já venceram
func (ar *AuctionRepository) closeExpiredAuctions() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	auctionInterval := getAuctionInterval()

	// Busca todos os leilões ativos
	activeAuctions, err := ar.FindAuctions(ctx, auction_entity.Active, "", "")
	if err != nil {
		logger.Error("Error trying to find active auctions", err)
		return
	}

	now := time.Now()
	closedCount := 0

	for _, auction := range activeAuctions {
		closeTime := auction.Timestamp.Add(auctionInterval)

		// Se o tempo de fechamento já passou, fecha o leilão
		if now.After(closeTime) {
			if updateErr := ar.CloseAuction(ctx, auction.Id); updateErr != nil {
				logger.Error("Error trying to close expired auction", updateErr)
			} else {
				logger.Info("Expired auction closed automatically: " + auction.Id)
				closedCount++
			}
		}
	}

	// Log estatísticas apenas quando há atividade
	if len(activeAuctions) > 0 || closedCount > 0 {
		logger.Info("Auction check completed - Active: " + string(rune(len(activeAuctions))) + ", Closed: " + string(rune(closedCount)))
	}
}

// getAuctionInterval retorna o intervalo de duração do leilão
func getAuctionInterval() time.Duration {
	// Prioridade: AUCTION_CLOSE_INTERVAL -> AUCTION_INTERVAL -> padrão
	auctionInterval := os.Getenv("AUCTION_CLOSE_INTERVAL")
	if auctionInterval == "" {
		auctionInterval = os.Getenv("AUCTION_INTERVAL")
	}

	duration, err := time.ParseDuration(auctionInterval)
	if err != nil {
		logger.Error("Invalid auction interval format, using default 5 minutes", err)
		return time.Minute * 5 // valor padrão de 5 minutos
	}

	return duration
}
