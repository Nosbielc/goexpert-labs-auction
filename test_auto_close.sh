#!/bin/bash

echo "🚀 Testando fechamento automático de leilões..."
echo "=============================================="

# 1. Verificar se a aplicação está rodando
echo "1. Verificando se a aplicação está ativa..."
if curl -s http://localhost:8080/auction > /dev/null; then
    echo "✅ Aplicação está rodando"
else
    echo "❌ Aplicação não está rodando. Execute: docker-compose up -d"
    exit 1
fi

# 2. Criar um leilão de teste
echo ""
echo "2. Criando leilão de teste..."
AUCTION_RESPONSE=$(curl -s -X POST http://localhost:8080/auction \
  -H "Content-Type: application/json" \
  -d '{
    "product_name": "Produto Teste Auto Close",
    "category": "Teste Automático",
    "description": "Produto criado para testar o fechamento automático do sistema",
    "condition": 1
  }')

echo "Resposta da criação: $AUCTION_RESPONSE"

# 3. Listar leilões ativos
echo ""
echo "3. Listando leilões ativos (status=0)..."
ACTIVE_AUCTIONS=$(curl -s "http://localhost:8080/auction?status=0")
echo "Leilões ativos: $ACTIVE_AUCTIONS"

# Contar quantos leilões ativos existem
ACTIVE_COUNT=$(echo $ACTIVE_AUCTIONS | jq '. | length' 2>/dev/null || echo "Não foi possível contar - verifique se jq está instalado")
echo "Quantidade de leilões ativos: $ACTIVE_COUNT"

# 4. Aguardar o fechamento (baseado no AUCTION_INTERVAL=30s)
echo ""
echo "4. Aguardando fechamento automático (30 segundos + margem)..."
echo "Tempo restante:"
for i in {35..1}; do
    printf "\r⏰ %02d segundos" $i
    sleep 1
done
echo ""

# 5. Verificar leilões após o tempo
echo ""
echo "5. Verificando leilões após o tempo de fechamento..."

echo "Leilões ativos (deve ter diminuído):"
ACTIVE_AFTER=$(curl -s "http://localhost:8080/auction?status=0")
echo "$ACTIVE_AFTER"

echo ""
echo "Leilões fechados (deve ter aumentado):"
COMPLETED_AFTER=$(curl -s "http://localhost:8080/auction?status=1")
echo "$COMPLETED_AFTER"

# 6. Verificar logs de fechamento
echo ""
echo "6. Últimos logs de fechamento automático:"
echo "======================================="
docker compose logs --tail=20 app | grep -i "auction closed automatically" || echo "Nenhum log de fechamento encontrado"

echo ""
echo "🏁 Teste concluído!"
echo "==================="
echo ""
echo "💡 Para acompanhar em tempo real:"
echo "   docker compose logs -f app | grep -i 'auction closed'"
echo ""
echo "📊 Para verificar status via API:"
echo "   curl 'http://localhost:8080/auction?status=0'  # Ativos"
echo "   curl 'http://localhost:8080/auction?status=1'  # Fechados"
