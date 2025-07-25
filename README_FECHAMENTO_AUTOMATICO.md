# Sistema de Leilões com Fechamento Automático

Este projeto implementa um sistema de leilões em Go com funcionalidade de fechamento automático baseado em tempo.

## Funcionalidades Implementadas

### ✅ Fechamento Automático de Leilões
- **Goroutines**: Implementação de rotinas concorrentes para monitoramento de leilões
- **Agendamento Inteligente**: Cada leilão é agendado para fechamento no momento exato
- **Verificação Periódica**: Sistema de backup que verifica leilões vencidos a cada minuto
- **Controle de Concorrência**: Uso de mutexes para operações thread-safe

### 🔧 Configuração de Tempo
- Configurável via variável de ambiente `AUCTION_INTERVAL`
- Valor padrão: 5 minutos se não especificado
- Suporte a diferentes formatos: `20s`, `5m`, `1h`, etc.

## Como Executar o Projeto

### Pré-requisitos
- Docker e Docker Compose instalados
- Go 1.20+ (para desenvolvimento local)

### 1. Executar com Docker Compose

```bash
# Clonar o repositório
git clone <repository-url>
cd goexpert-labs-auction

# Subir todos os serviços
docker-compose up -d

# Verificar logs da aplicação
docker-compose logs -f app

# Verificar logs do MongoDB
docker-compose logs -f mongodb
```

### 2. Executar Localmente para Desenvolvimento

```bash
# Subir apenas o MongoDB
docker-compose up -d mongodb

# Instalar dependências
go mod tidy

# Executar a aplicação
go run cmd/auction/main.go
```

### 3. Executar Testes

```bash
# Subir MongoDB para testes
docker-compose up -d mongodb

# Executar todos os testes
go test ./...

# Executar testes específicos do fechamento automático
go test ./internal/infra/database/auction/ -v

# Executar teste específico de auto-close
go test ./internal/infra/database/auction/ -run TestAuctionAutoClose -v
```

## Configuração de Variáveis de Ambiente

O arquivo `cmd/auction/.env` contém as configurações:

```env
# Configurações do leilão
AUCTION_INTERVAL=20s          # Tempo de duração dos leilões
BATCH_INSERT_INTERVAL=20s     # Intervalo para inserção em lote
MAX_BATCH_SIZE=4              # Tamanho máximo do lote

# Configurações do MongoDB
MONGO_INITDB_ROOT_USERNAME=admin
MONGO_INITDB_ROOT_PASSWORD=admin
MONGODB_URL=mongodb://admin:admin@mongodb:27017/auctions?authSource=admin
MONGODB_DB=auctions
```

## Endpoints da API

### Criar Leilão
```http
POST http://localhost:8080/auction
Content-Type: application/json

{
  "product_name": "Produto Teste",
  "category": "Eletrônicos", 
  "description": "Descrição detalhada do produto",
  "condition": 1
}
```

### Listar Leilões
```http
GET http://localhost:8080/auction?status=0&category=Eletrônicos
```

### Criar Lance
```http
POST http://localhost:8080/bid
Content-Type: application/json

{
  "user_id": "user123",
  "auction_id": "auction_id_aqui",
  "amount": 100.50
}
```

## Testando o Fechamento Automático

### Teste Manual Rápido

1. **Configure intervalo curto**:
   ```bash
   # Edite o arquivo .env
   AUCTION_INTERVAL=30s
   ```

2. **Reinicie a aplicação**:
   ```bash
   docker-compose restart app
   ```

3. **Crie um leilão**:
   ```bash
   curl -X POST http://localhost:8080/auction \
     -H "Content-Type: application/json" \
     -d '{
       "product_name": "Produto Teste Auto Close",
       "category": "Teste",
       "description": "Produto para testar fechamento automático",
       "condition": 1
     }'
   ```

4. **Verifique o status inicial**:
   ```bash
   curl http://localhost:8080/auction?status=0
   ```

5. **Aguarde 30 segundos e verifique novamente**:
   ```bash
   curl http://localhost:8080/auction
   ```

### Monitoramento via Logs

```bash
# Acompanhe os logs da aplicação para ver os fechamentos automáticos
docker-compose logs -f app | grep -i "auction closed automatically"
```

## Arquitetura da Solução

### Componentes Principais

1. **AuctionRepository**: Gerencia operações de leilão no banco
2. **scheduleAuctionClose**: Agenda fechamento específico por leilão
3. **startAuctionCloserRoutine**: Goroutine de verificação periódica
4. **closeExpiredAuctions**: Fecha leilões vencidos em lote

### Estratégia de Fechamento Dupla

1. **Agendamento Individual**: Cada leilão agenda seu próprio fechamento
2. **Verificação Periódica**: Sistema de backup verifica a cada minuto

### Controle de Concorrência

- **sync.Mutex**: Protege mapas compartilhados
- **sync.Once**: Garante que a goroutine de monitoramento inicie apenas uma vez
- **Context**: Controla timeouts e cancelamentos

## Solução de Problemas

### MongoDB não conecta
```bash
# Verificar se o MongoDB está rodando
docker-compose ps mongodb

# Verificar logs do MongoDB
docker-compose logs mongodb
```

### Testes falhando
```bash
# Garantir que o MongoDB está disponível na porta 27017
netstat -an | grep 27017

# Reinstalar dependências
go mod tidy
```

### Leilões não fecham automaticamente
```bash
# Verificar logs da aplicação
docker-compose logs app | grep -i error

# Verificar configuração da variável AUCTION_INTERVAL
docker-compose exec app env | grep AUCTION_INTERVAL
```

## Monitoramento e Debug

### Logs Importantes
- `"Auction closed automatically"`: Fechamento por agendamento
- `"Expired auction closed automatically"`: Fechamento por verificação periódica
- `"Error trying to close auction"`: Erros no fechamento

### Métricas de Performance
- Tempo entre criação e fechamento do leilão
- Número de leilões fechados automaticamente vs. manualmente
- Eficiência da goroutine de verificação periódica
