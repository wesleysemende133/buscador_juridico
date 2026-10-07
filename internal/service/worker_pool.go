package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ============================================
// WORKER POOL
// ============================================

// Job representa um pedido do utilizador
type Job struct {
	ID         string
	Pergunta   string
	Modo       string
	Email      string
	ResultChan chan JobResult
	Ctx        context.Context
}

// JobResult devolve o resultado do processamento
type JobResult struct {
	Resposta           string
	Artigos            []string
	PerguntasRestantes int
	Erro               error
	Duracao            time.Duration
}

// WorkerPool gere os workers e a fila
type WorkerPool struct {
	workers    int
	fila       chan *Job
	agente     *AgenteService
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
	metricas   *MetricasPool
}

// MetricasPool guarda estatísticas
type MetricasPool struct {
	mu               sync.Mutex
	TotalProcessados int
	TotalErros       int
	TotalEspera      time.Duration
	EmFila           int
	Processando      int
}

// NovoWorkerPool cria um pool de workers
func NovoWorkerPool(agente *AgenteService, workers int, capacidadeFila int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	pool := &WorkerPool{
		workers:  workers,
		fila:     make(chan *Job, capacidadeFila),
		agente:   agente,
		ctx:      ctx,
		cancel:   cancel,
		metricas: &MetricasPool{},
	}

	// Iniciar workers
	for i := 1; i <= workers; i++ {
		pool.wg.Add(1)
		go pool.worker(i)
	}

	log.Printf("👷 Worker Pool iniciado: %d workers, fila de %d", workers, capacidadeFila)

	return pool
}

// worker processa jobs da fila
func (p *WorkerPool) worker(id int) {
	defer p.wg.Done()

	log.Printf("👷 Worker %d iniciado", id)

	for {
		select {
		case <-p.ctx.Done():
			log.Printf("👷 Worker %d finalizado (contexto cancelado)", id)
			return
		case job, ok := <-p.fila:
			if !ok {
				log.Printf("👷 Worker %d finalizado (fila fechada)", id)
				return
			}

			p.metricas.mu.Lock()
			p.metricas.EmFila--
			p.metricas.Processando++
			p.metricas.mu.Unlock()

			inicio := time.Now()

			// Processar o job
			resultado := p.processar(job)

			resultado.Duracao = time.Since(inicio)

			p.metricas.mu.Lock()
			p.metricas.Processando--
			if resultado.Erro != nil {
				p.metricas.TotalErros++
			} else {
				p.metricas.TotalProcessados++
			}
			p.metricas.mu.Unlock()

			// Devolver resultado
			select {
			case job.ResultChan <- resultado:
			case <-job.Ctx.Done():
				// Cliente desistiu
				log.Printf("⚠️  Worker %d: cliente desistiu do job %s", id, job.ID)
			case <-time.After(5 * time.Second):
				// Timeout a devolver
				log.Printf("⚠️  Worker %d: timeout a devolver job %s", id, job.ID)
			}
		}
	}
}

// processar executa o AgenteService
func (p *WorkerPool) processar(job *Job) JobResult {
	req := AgenteRequest{
		Pergunta: job.Pergunta,
		Modo:     job.Modo,
		Email:    job.Email,
	}

	resposta, err := p.agente.Processar(job.Ctx, req)
	if err != nil {
		return JobResult{Erro: err}
	}

	// Extrair IDs dos artigos
	artigos := make([]string, 0, len(resposta.Artigos))
	for _, a := range resposta.Artigos {
		artigos = append(artigos, a.Artigo)
	}

	return JobResult{
		Resposta:           resposta.Resposta,
		Artigos:            artigos,
		PerguntasRestantes: resposta.PerguntasRestantes,
	}
}

// ============================================
// API PÚBLICA
// ============================================

// Submeter envia um job para a fila
func (p *WorkerPool) Submeter(ctx context.Context, pergunta, modo, email string) (*JobResult, error) {
	job := &Job{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		Pergunta:   pergunta,
		Modo:       modo,
		Email:      email,
		ResultChan: make(chan JobResult, 1),
		Ctx:        ctx,
	}

	p.metricas.mu.Lock()
	p.metricas.EmFila++
	emFila := p.metricas.EmFila
	p.metricas.mu.Unlock()

	log.Printf("📥 Job %s submetido (fila: %d)", job.ID, emFila)

	// Tentar enviar para a fila
	select {
	case p.fila <- job:
		// Sucesso
	case <-ctx.Done():
		p.metricas.mu.Lock()
		p.metricas.EmFila--
		p.metricas.mu.Unlock()
		return nil, ctx.Err()
	case <-time.After(2 * time.Second):
		// Fila cheia
		p.metricas.mu.Lock()
		p.metricas.EmFila--
		p.metricas.mu.Unlock()
		return nil, fmt.Errorf("sistema ocupado, tenta novamente")
	}

	// Esperar pelo resultado
	select {
	case resultado := <-job.ResultChan:
		return &resultado, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(120 * time.Second):
		return nil, fmt.Errorf("timeout ao processar pedido")
	}
}

// Metricas devolve as estatísticas
func (p *WorkerPool) Metricas() MetricasPool {
	p.metricas.mu.Lock()
	defer p.metricas.mu.Unlock()
	return MetricasPool{
		TotalProcessados: p.metricas.TotalProcessados,
		TotalErros:       p.metricas.TotalErros,
		EmFila:           p.metricas.EmFila,
		Processando:      p.metricas.Processando,
	}
}

// Parar encerra o pool
func (p *WorkerPool) Parar() {
	log.Println("🛑 A parar Worker Pool...")
	p.cancel()
	close(p.fila)
	p.wg.Wait()
	log.Println("✅ Worker Pool parado")
}
