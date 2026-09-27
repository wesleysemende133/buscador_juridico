-- ============================================
-- SCHEMA DO BUSCADOR JURÍDICO
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

CREATE TABLE IF NOT EXISTS usuarios (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    senha_hash TEXT NOT NULL,
    nome TEXT,
    tipo TEXT DEFAULT 'cidadao',
    ativo BOOLEAN DEFAULT TRUE,
    criado_em TIMESTAMP DEFAULT NOW(),
    ultimo_login TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pesquisas (
    id SERIAL PRIMARY KEY,
    query TEXT NOT NULL,
    categoria TEXT,
    resultados INTEGER,
    usuario_id INTEGER REFERENCES usuarios(id),
    ip TEXT,
    criado_em TIMESTAMP DEFAULT NOW()
);

CREATE OR REPLACE VIEW vw_estatisticas AS
SELECT
    COUNT(*) as total_artigos,
    COUNT(DISTINCT lei_numero) as total_leis,
    COUNT(DISTINCT categoria) as total_categorias,
    COUNT(CASE WHEN status = 'Vigente' THEN 1 END) as vigentes
FROM artigos;

SELECT 'Schema criado com sucesso!' as mensagem;
