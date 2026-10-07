import { useState, useCallback, useRef } from 'react';

interface EventoStream {
  tipo: 'progresso' | 'token' | 'artigos' | 'fim' | 'erro' | 'precisa_login' | 'limite_atingido';
  dados: any;
  timestamp: string;
}

interface EstadoStream {
  progresso: string;
  texto: string;
  artigos: any[];
  carregando: boolean;
  erro: string;
  perguntasRestantes: number;
}

export function useAgenteStream() {
  const [estado, setEstado] = useState<EstadoStream>({
    progresso: '',
    texto: '',
    artigos: [],
    carregando: false,
    erro: '',
    perguntasRestantes: -1,
  });

  const abortRef = useRef<AbortController | null>(null);

  const enviar = useCallback(async (pergunta: string, modo: string) => {
    // Reset
    setEstado({
      progresso: '',
      texto: '',
      artigos: [],
      carregando: true,
      erro: '',
      perguntasRestantes: -1,
    });

    // Abortar pedido anterior
    if (abortRef.current) {
      abortRef.current.abort();
    }
    abortRef.current = new AbortController();

    const token = localStorage.getItem('base_legal_token');
    const baseURL = import.meta.env.VITE_API_URL || '/api';

    try {
      const resposta = await fetch(`${baseURL}/agente/stream`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify({ pergunta, modo }),
        signal: abortRef.current.signal,
      });

      if (!resposta.ok) {
        throw new Error(`HTTP ${resposta.status}`);
      }

      const reader = resposta.body?.getReader();
      if (!reader) throw new Error('Sem body');

      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });

        // Processar linhas SSE
        const linhas = buffer.split('\n\n');
        buffer = linhas.pop() || '';

        for (const linha of linhas) {
          if (!linha.startsWith('data: ')) continue;

          const json = linha.substring(6);
          try {
            const evento: EventoStream = JSON.parse(json);

            setEstado((prev) => {
              switch (evento.tipo) {
                case 'progresso':
                  return { ...prev, progresso: evento.dados.mensagem };

                case 'token':
                  return { ...prev, texto: prev.texto + (evento.dados.texto || '') };

                case 'artigos':
                  return { ...prev, artigos: evento.dados.artigos || [] };

                case 'fim':
                  return {
                    ...prev,
                    texto: evento.dados.resposta || prev.texto,
                    artigos: evento.dados.artigos || [],
                    perguntasRestantes: evento.dados.perguntas_restantes ?? -1,
                    carregando: false,
                    progresso: '',
                  };

                case 'erro':
                  return { ...prev, erro: evento.dados.mensagem, carregando: false };

                case 'precisa_login':
                  return { ...prev, erro: evento.dados.mensagem, carregando: false };

                case 'limite_atingido':
                  return { ...prev, erro: evento.dados.mensagem, carregando: false };

                default:
                  return prev;
              }
            });
          } catch (e) {
            console.error('Erro a parsear evento:', e);
          }
        }
      }
    } catch (err: any) {
      if (err.name === 'AbortError') return;
      setEstado((prev) => ({
        ...prev,
        erro: err.message || 'Erro ao processar',
        carregando: false,
      }));
    }
  }, []);

  const parar = useCallback(() => {
    if (abortRef.current) {
      abortRef.current.abort();
      abortRef.current = null;
    }
    setEstado((prev) => ({ ...prev, carregando: false }));
  }, []);

  return { estado, enviar, parar };
}
