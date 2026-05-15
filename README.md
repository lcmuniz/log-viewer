# Log Viewer - Monitor de Logs Web

Aplicação web em Go para monitorar arquivos de log em tempo real.

## Características

- Interface web moderna e responsiva
- Múltiplas abas para visualizar vários logs simultaneamente
- Refresh automático configurável
- Auto-scroll para acompanhar novos logs
- Suporte a arquivos .log, .txt e sem extensão
- Tema claro/escuro

## Instalação

```bash
make install
```

Ou manualmente:
```bash
go install ./cmd/log-viewer
```

## Como usar

Execute o log-viewer:
```bash
log-viewer
```

Ou com parâmetros:
```bash
log-viewer -r 5 -l /Users/lcmuniz/logs -t light
```

### Parâmetros disponíveis:
- `-r`: Intervalo de refresh em segundos (padrão: 5)
- `-l`: Caminho da pasta de logs (padrão: pasta atual)
- `-t`: Tema - "light" ou "dark" (padrão: light)

O arquivo `log-viewer.json` será criado automaticamente na pasta de logs especificada.

### Abra o navegador em: http://localhost:3000

## Configuração

O arquivo `log-viewer.json` é criado automaticamente com valores padrão:

```json
{
  "logFolder": "/pasta/atual",
  "refreshInterval": 5,
  "theme": "light"
}
```

- **logFolder**: Caminho completo da pasta contendo os arquivos de log
- **refreshInterval**: Intervalo em segundos para atualização automática
- **theme**: Tema da interface - "light" ou "dark"

## Desenvolvimento

Build local:
```bash
make build
./bin/log-viewer
```

Executar diretamente:
```bash
make run
```

Limpar builds:
```bash
make clean
```

## Requisitos

- Go 1.21 ou superior
- Permissão de leitura na pasta de logs
