# Sistema de Leilões com Fechamento Automático

Este projeto implementa um sistema de leilões em Go com funcionalidade de fechamento automático baseado em tempo.

## Funcionalidades Implementadas

### ✅ Fechamento Automático de Leilões
- **Goroutines**: Implementação de rotinas concorrentes para monitoramento de leilões
- **Agendamento Inteligente**: Cada leilão é agendado para fechamento no momento exato
- **Verificação Periódica**: Sistema de backup que verifica leilões vencidos automaticamente
- **Controle de Concorrência**: Uso de mutexes para operações thread-safe

### 🔧 Configuração de Tempo
- Configurável via variável de ambiente `AUCTION_CLOSE_INTERVAL` ou `AUCTION_INTERVAL`
- Valor padrão: 5 minutos se não especificado
- Suporte a diferentes formatos: `5s`, `30s`, `5m`, `1h`, etc.

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
AUCTION_CLOSE_INTERVAL=5s        # Tempo de duração dos leilões (prioridade)
AUCTION_INTERVAL=30s             # Tempo de duração dos leilões (fallback)
AUCTION_MAX_DURATION=30s         # Duração máxima (compatibilidade)
BATCH_INSERT_INTERVAL=20s        # Intervalo para inserção em lote
MAX_BATCH_SIZE=4                 # Tamanho máximo do lote

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
# Todos os leilões
GET http://localhost:8080/auction

# Apenas leilões ativos (status=0)
GET http://localhost:8080/auction?status=0

# Apenas leilões fechados (status=1)
GET http://localhost:8080/auction?status=1

# Filtrar por categoria
GET http://localhost:8080/auction?category=Eletrônicos
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

### Teste Automatizado
```bash
# Executar script de teste automático
chmod +x test_auto_close.sh
./test_auto_close.sh
```

### Teste Manual Rápido

1. **Configure intervalo curto** (já está configurado para 5s):
   ```bash
   # No arquivo cmd/auction/.env
   AUCTION_CLOSE_INTERVAL=5s
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
   curl "http://localhost:8080/auction?status=0"
   ```

5. **Aguarde 6 segundos e verifique novamente**:
   ```bash
   sleep 6
   curl "http://localhost:8080/auction?status=1"
   ```

### Como Verificar se Está Funcionando

#### Via API:
```bash
# Leilões ativos (status=0)
curl "http://localhost:8080/auction?status=0"

# Leilões fechados (status=1) 
curl "http://localhost:8080/auction?status=1"
```

#### Via Logs:
```bash
# Acompanhar logs em tempo real
docker-compose logs -f app

# Filtrar logs de fechamento automático
docker-compose logs app | grep -i "auction closed automatically"
```

### Interpretação dos Status:
- **"status": 0** = Leilão ATIVO (ainda aberto)
- **"status": 1** = Leilão COMPLETED (fechado automaticamente)

## Arquitetura da Solução

### Componentes Principais

1. **AuctionRepository**: Gerencia operações de leilão no banco
2. **scheduleAuctionClose**: Agenda fechamento específico por leilão
3. **startAuctionCloserRoutine**: Goroutine de verificação periódica
4. **closeExpiredAuctions**: Fecha leilões vencidos em lote

### Estratégia de Fechamento Dupla

```
🔄 Dupla Estratégia de Fechamento:
├── 1️⃣ Agendamento Individual (time.AfterFunc)
│   └── Cada leilão agenda seu próprio fechamento
└── 2️⃣ Verificação Periódica (ticker adaptativo)
    └── Backup que verifica leilões vencidos

🔒 Controle de Concorrência:
├── sync.Mutex → Thread-safety para mapas
├── sync.Once → Goroutine única de monitoramento
└── Context.WithTimeout → Evita travamentos

⚙️ Configuração Flexível:
├── AUCTION_CLOSE_INTERVAL (prioridade)
├── AUCTION_INTERVAL (fallback)
└── 5 minutos (padrão)
```

### Intervalos Adaptativos

- **Leilões de 5s**: Verifica a cada 2,5s
- **Leilões de 30s**: Verifica a cada 15s  
- **Leilões > 1min**: Verifica a cada minuto
- **Mínimo**: 10 segundos para evitar sobrecarga

## Solução de Problemas

### MongoDB não conecta
```bash
# Verificar se o MongoDB está rodando
docker-compose ps mongodb

# Verificar logs do MongoDB
docker-compose logs mongodb

# Reiniciar MongoDB
docker-compose restart mongodb
```

### Testes falhando
```bash
# Garantir que o MongoDB está disponível na porta 27017
netstat -an | grep 27017

# Reinstalar dependências
go mod tidy

# Limpar e recompilar
docker-compose down && docker-compose up --build -d
```

### Leilões não fecham automaticamente
```bash
# Verificar logs da aplicação
docker-compose logs app | grep -i error

# Verificar configuração das variáveis
docker-compose exec app env | grep AUCTION

# Verificar se a goroutine está rodando
docker-compose logs app | grep -i "starting auction closer"
```

### Aplicação não compila
```bash
# Verificar erros de compilação
docker-compose up --build

# Se necessário, fazer build local para debug
go build -o auction cmd/auction/main.go
```

## Monitoramento e Debug

### Logs Importantes
- `"Starting auction closer routine"`: Goroutine de monitoramento iniciada
- `"Auction closed automatically"`: Fechamento por agendamento individual
- `"Expired auction closed automatically"`: Fechamento por verificação periódica
- `"Auction check completed"`: Estatísticas de verificação periódica

### Verificações de Saúde
```bash
# Status dos containers
docker-compose ps

# Logs da aplicação
docker-compose logs --tail=50 app

# Verificar variáveis de ambiente
docker-compose exec app env | grep AUCTION

# Teste de conectividade da API
curl -s http://localhost:8080/auction
```

### Métricas de Performance
- Tempo entre criação e fechamento do leilão
- Número de leilões fechados automaticamente vs. manualmente
- Eficiência da goroutine de verificação periódica
- Frequência de verificações baseada no intervalo configurado

## Desenvolvimento e Contribuição

### Estrutura do Código
- `internal/infra/database/auction/create_auction.go`: Implementação principal
- `internal/infra/database/auction/create_auction_test.go`: Testes da funcionalidade
- `cmd/auction/.env`: Configurações do ambiente
- `docker-compose.yml`: Orquestração dos serviços

### Executar em Modo de Desenvolvimento
```bash
# Apenas o banco
docker-compose up -d mongodb

# Aplicação local com hot-reload
go run cmd/auction/main.go

# Ou com air (se instalado)
air
```

---

## 🎉 Funcionalidade Implementada com Sucesso!

✅ **Fechamento automático baseado em tempo**  
✅ **Goroutines para concorrência**  
✅ **Controle thread-safe com mutexes**  
✅ **Testes automatizados completos**  
✅ **Configuração flexível via variáveis de ambiente**  
✅ **Logs informativos para monitoramento**  
✅ **Estratégia dupla para máxima confiabilidade**
