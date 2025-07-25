package auction

import (
	"context"
	"fullcycle-auction_go/internal/entity/auction_entity"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestAuctionAutoClose(t *testing.T) {
	// Define um intervalo de 3 segundos para o teste
	os.Setenv("AUCTION_INTERVAL", "3s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	// Conecta ao MongoDB de teste
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://admin:admin@localhost:27017"))
	if err != nil {
		t.Skip("MongoDB não disponível para teste")
	}
	defer client.Disconnect(ctx)

	database := client.Database("test_auctions")
	defer database.Drop(ctx)

	repo := NewAuctionRepository(database)

	// Cria um leilão de teste
	auction, createErr := auction_entity.CreateAuction(
		"Produto Teste Auto Close",
		"Categoria Teste",
		"Descrição do produto de teste para validação de fechamento automático",
		auction_entity.New,
	)
	if createErr != nil {
		t.Fatalf("Erro ao criar entidade leilão: %v", createErr)
	}

	// Cria o leilão no banco
	insertErr := repo.CreateAuction(ctx, auction)
	if insertErr != nil {
		t.Fatalf("Erro ao inserir leilão: %v", insertErr)
	}

	// Verifica se o leilão está ativo inicialmente
	foundAuction, findErr := repo.FindAuctionById(ctx, auction.Id)
	if findErr != nil {
		t.Fatalf("Erro ao buscar leilão: %v", findErr)
	}

	if foundAuction.Status != auction_entity.Active {
		t.Fatalf("Leilão deveria estar ativo, mas está: %v", foundAuction.Status)
	}

	t.Logf("Leilão criado com sucesso. ID: %s, Status: %v", auction.Id, foundAuction.Status)

	// Aguarda o tempo do leilão + margem para garantir o fechamento
	t.Log("Aguardando fechamento automático do leilão...")
	time.Sleep(5 * time.Second)

	// Verifica se o leilão foi fechado automaticamente
	foundAuction, findErr = repo.FindAuctionById(ctx, auction.Id)
	if findErr != nil {
		t.Fatalf("Erro ao buscar leilão após fechamento: %v", findErr)
	}

	if foundAuction.Status != auction_entity.Completed {
		t.Fatalf("Leilão deveria estar fechado (Completed), mas está: %v", foundAuction.Status)
	}

	t.Log("✅ Teste de fechamento automático passou com sucesso!")
}

func TestAuctionIntervalCalculation(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected time.Duration
	}{
		{
			name:     "Intervalo válido de 5 minutos",
			envValue: "5m",
			expected: 5 * time.Minute,
		},
		{
			name:     "Intervalo válido de 30 segundos",
			envValue: "30s",
			expected: 30 * time.Second,
		},
		{
			name:     "Intervalo inválido deve retornar padrão",
			envValue: "invalid",
			expected: 5 * time.Minute,
		},
		{
			name:     "Sem variável de ambiente deve retornar padrão",
			envValue: "",
			expected: 5 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv("AUCTION_INTERVAL", tt.envValue)
				defer os.Unsetenv("AUCTION_INTERVAL")
			} else {
				os.Unsetenv("AUCTION_INTERVAL")
			}

			interval := getAuctionInterval()

			if interval != tt.expected {
				t.Errorf("Intervalo esperado: %v, obtido: %v", tt.expected, interval)
			}
		})
	}
}

func TestScheduleAuctionClose(t *testing.T) {
	// Define um intervalo muito curto para o teste
	os.Setenv("AUCTION_INTERVAL", "1s")
	defer os.Unsetenv("AUCTION_INTERVAL")

	ctx := context.Background()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://admin:admin@localhost:27017"))
	if err != nil {
		t.Skip("MongoDB não disponível para teste")
	}
	defer client.Disconnect(ctx)

	database := client.Database("test_schedule")
	defer database.Drop(ctx)

	repo := NewAuctionRepository(database)

	// Cria um leilão que já deveria estar vencido
	pastTime := time.Now().Add(-2 * time.Second)
	auction, _ := auction_entity.CreateAuction(
		"Produto Vencido",
		"Categoria Teste",
		"Produto que já deveria estar fechado",
		auction_entity.Used,
	)
	auction.Timestamp = pastTime

	// Insere manualmente no banco para simular um leilão criado no passado
	auctionMongo := &AuctionEntityMongo{
		Id:          auction.Id,
		ProductName: auction.ProductName,
		Category:    auction.Category,
		Description: auction.Description,
		Condition:   auction.Condition,
		Status:      auction_entity.Active,
		Timestamp:   pastTime.Unix(),
	}

	_, insertErr := repo.Collection.InsertOne(ctx, auctionMongo)
	if insertErr != nil {
		t.Fatalf("Erro ao inserir leilão: %v", insertErr)
	}

	// Agenda o fechamento (deve fechar imediatamente)
	repo.scheduleAuctionClose(auction.Id, pastTime)

	// Aguarda um pouco para a goroutine processar
	time.Sleep(100 * time.Millisecond)

	// Verifica se foi fechado
	foundAuction, findErr := repo.FindAuctionById(ctx, auction.Id)
	if findErr != nil {
		t.Fatalf("Erro ao buscar leilão: %v", findErr)
	}

	if foundAuction.Status != auction_entity.Completed {
		t.Fatalf("Leilão vencido deveria estar fechado, mas está: %v", foundAuction.Status)
	}

	t.Log("✅ Teste de agendamento de fechamento passou com sucesso!")
}
