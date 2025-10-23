# Correção de Timeout do Bot do Telegram

## Problema Identificado

A aplicação estava apresentando erros intermitentes de timeout na função `getUpdates` do bot do Telegram:

```
2025/10/20 20:32:31 [TGBOT] [ERROR] error get updates, error do request for method getUpdates, Post "https://api.telegram.org/bot***/getUpdates": context deadline exceeded (Client.Timeout exceeded while awaiting headers)
```

## Causa Raiz

O problema estava relacionado a:

1. **Timeout inadequado**: A biblioteca `go-telegram/bot` usa um timeout padrão de 1 minuto que pode ser insuficiente em condições de rede instáveis
2. **Falta de retry robusto**: O mecanismo de retry existente não era suficiente para lidar com falhas temporárias de rede
3. **Contextos sem timeout**: Algumas operações não tinham timeouts de contexto adequados

## Soluções Implementadas

### 1. Mecanismo de Retry com Backoff Exponencial

Implementado em `telegram_handler/telegram.go`:

- **Função `retryWithBackoff`**: Executa operações com retry automático
- **Backoff exponencial**: Aumenta o tempo de espera entre tentativas (2s, 4s, 8s, etc.)
- **Cap de 30 segundos**: Limita o tempo máximo de espera
- **Detecção de erros temporários**: Identifica erros de rede que podem ser temporários

### 2. Detecção Inteligente de Erros Temporários

Função `isTemporaryError` que identifica:
- Erros de rede (timeout, connection refused, etc.)
- Erros de contexto (deadline exceeded)
- Padrões específicos em mensagens de erro

### 3. Aplicação de Retry em Todas as Operações

Todas as funções de envio agora usam retry:
- `SendMessage`: 3 tentativas com retry
- `SendPhoto`: 3 tentativas com retry  
- `SendVideo`: 3 tentativas com retry

### 4. Timeouts de Contexto Melhorados

- **Fotos**: Timeout aumentado para 2 minutos
- **Mensagem de startup**: Timeout de 30 segundos
- **Contextos adequados**: Todas as operações têm timeouts apropriados

## Benefícios

1. **Maior robustez**: A aplicação agora lida melhor com falhas temporárias de rede
2. **Recuperação automática**: Retry automático sem intervenção manual
3. **Logs informativos**: Logs detalhados sobre tentativas de retry
4. **Timeouts apropriados**: Evita operações que ficam "penduradas" indefinidamente

## Configurações

### Retry Configuration
- **Máximo de tentativas**: 3
- **Delay base**: 2 segundos
- **Backoff exponencial**: 2s → 4s → 8s
- **Cap máximo**: 30 segundos

### Timeout Configuration
- **Fotos**: 2 minutos
- **Vídeos**: 2 minutos (já existente)
- **Mensagens**: 30 segundos
- **Startup**: 30 segundos

## Monitoramento

A aplicação agora registra:
- Tentativas de retry com timing
- Erros temporários vs permanentes
- Sucesso/falha de operações

## Exemplo de Logs

```
2025/10/20 20:32:31 [TGBOT] [ERROR] Temporary error on attempt 1: context deadline exceeded
2025/10/20 20:32:33 Retrying in 2s (attempt 2/3)
2025/10/20 20:32:35 Photo for event abc123 sent to Telegram successfully.
```

## Testes Recomendados

1. **Teste de conectividade instável**: Simular falhas de rede
2. **Teste de timeout**: Verificar comportamento com timeouts
3. **Teste de retry**: Confirmar que retry funciona corretamente
4. **Teste de recuperação**: Verificar se aplicação se recupera automaticamente

## Considerações Futuras

- Monitorar métricas de retry para ajustar configurações
- Considerar implementar circuit breaker para falhas persistentes
- Adicionar métricas de performance das operações
