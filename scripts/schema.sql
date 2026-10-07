-- ============================================
-- SCHEMA DO BUSCADOR JURÍDICO
-- Última actualização: 2026-09-30
-- ============================================

-- ============================================
-- ARTIGOS
-- ============================================
CREATE TABLE IF NOT EXISTS artigos (
    id TEXT PRIMARY KEY,
    lei TEXT NOT NULL,
    lei_numero TEXT NOT NULL,
    artigo TEXT NOT NULL,
    texto TEXT NOT NULL,
    palavras_chave TEXT[],
    categoria TEXT,
    subcategoria TEXT,
    versao INTEGER DEFAULT 1,
    data_vigencia TIMESTAMP,
    data_publicacao TIMESTAMP,
    status TEXT DEFAULT 'Vigente',
    status_motivo TEXT,
    fonte TEXT,
    url TEXT,
    criado_em TIMESTAMP DEFAULT NOW(),
    atualizado_em TIMESTAMP DEFAULT NOW(),
    aprovado_por TEXT
);

CREATE INDEX IF NOT EXISTS idx_artigos_lei ON artigos(lei);
CREATE INDEX IF NOT EXISTS idx_artigos_lei_numero ON artigos(lei_numero);
CREATE INDEX IF NOT EXISTS idx_artigos_categoria ON artigos(categoria);
CREATE INDEX IF NOT EXISTS idx_artigos_subcategoria ON artigos(subcategoria);
CREATE INDEX IF NOT EXISTS idx_artigos_status ON artigos(status);
CREATE INDEX IF NOT EXISTS idx_artigos_artigo ON artigos(artigo);

CREATE INDEX IF NOT EXISTS idx_artigos_texto_fts ON artigos 
    USING GIN(to_tsvector('portuguese', texto));

CREATE INDEX IF NOT EXISTS idx_artigos_lei_fts ON artigos 
    USING GIN(to_tsvector('portuguese', lei));

-- ============================================
-- HISTÓRICO DE ARTIGOS (auditoria jurídica)
-- ============================================
CREATE TABLE IF NOT EXISTS historico_artigos (
    id SERIAL PRIMARY KEY,
    artigo_id TEXT REFERENCES artigos(id) ON DELETE CASCADE,
    versao INTEGER,
    texto_anterior TEXT,
    texto_novo TEXT,
    data_mudanca TIMESTAMP DEFAULT NOW(),
    motivo TEXT,
    alterado_por TEXT
);

CREATE INDEX IF NOT EXISTS idx_historico_artigo_id ON historico_artigos(artigo_id);

-- ============================================
-- UTILIZADORES
-- ============================================
CREATE TABLE IF NOT EXISTS utilizadores (
    id SERIAL PRIMARY KEY,
    nome VARCHAR(100) NOT NULL,
    email VARCHAR(150) UNIQUE NOT NULL,
    senha_hash VARCHAR(255) NOT NULL,
    role VARCHAR(20) NOT NULL DEFAULT 'user',
    oauth_provider VARCHAR(20),
    oauth_sub VARCHAR(255),
    criado_em TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_utilizadores_email ON utilizadores(email);
CREATE INDEX IF NOT EXISTS idx_utilizadores_oauth ON utilizadores(oauth_provider, oauth_sub);

-- ============================================
-- USO DO AGENTE IA
-- ============================================
CREATE TABLE IF NOT EXISTS uso_agente (
    id SERIAL PRIMARY KEY,
    user_id INT REFERENCES utilizadores(id) ON DELETE CASCADE,
    email VARCHAR(150) NOT NULL,
    modo VARCHAR(20) DEFAULT 'cidadao',
    contagem INT DEFAULT 0,
    plano VARCHAR(20) DEFAULT 'gratuito',
    primeira_uso TIMESTAMP DEFAULT NOW(),
    ultimo_uso TIMESTAMP DEFAULT NOW(),
    UNIQUE(email, modo)
);

CREATE INDEX IF NOT EXISTS idx_uso_email ON uso_agente(email);

-- ============================================
-- SOLICITAÇÕES DE ADVOGADO
-- ============================================
CREATE TABLE IF NOT EXISTS solicitacoes_advogado (
    id SERIAL PRIMARY KEY,
    nome VARCHAR(150) NOT NULL,
    email VARCHAR(150) NOT NULL,
    telefone VARCHAR(50),
    area_direito VARCHAR(100),
    descricao TEXT NOT NULL,
    urgencia VARCHAR(20) DEFAULT 'media',
    status VARCHAR(20) DEFAULT 'pendente',
    criado_em TIMESTAMP DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_solicitacoes_email ON solicitacoes_advogado(email);
CREATE INDEX IF NOT EXISTS idx_solicitacoes_status ON solicitacoes_advogado(status);

-- ============================================
-- VIEW DE ESTATÍSTICAS
-- ============================================
CREATE OR REPLACE VIEW vw_estatisticas AS
SELECT
    COUNT(*) as total_artigos,
    COUNT(DISTINCT lei_numero) as total_leis,
    COUNT(DISTINCT categoria) as total_categorias,
    COUNT(CASE WHEN status = 'Vigente' THEN 1 END) as vigentes
FROM artigos;

-- ============================================
-- FIM
-- ============================================
SELECT 'Schema criado com sucesso!' as mensagem;
