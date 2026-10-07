#!/bin/bash
# OCR em PDFs digitalizados
# Uso: ./scripts/ocr_pdfs.sh data/pdfs data/ocr

INPUT_DIR="${1:-data/pdfs}"
OUTPUT_DIR="${2:-data/ocr}"

mkdir -p "$OUTPUT_DIR"

echo "📂 Input:  $INPUT_DIR"
echo "📂 Output: $OUTPUT_DIR"
echo ""

processados=0
ignorados=0
falhados=0

for pdf in "$INPUT_DIR"/*.pdf; do
    [ -f "$pdf" ] || continue

    nome=$(basename "$pdf" .pdf)
    output="$OUTPUT_DIR/$nome.txt"

    # Ignorar se já existe
    if [ -f "$output" ]; then
        echo "⏭️  Ignorar (já existe): $nome"
        ignorados=$((ignorados + 1))
        continue
    fi

    echo "🔍 OCR: $nome"

    # Limpar ficheiros temporários
    rm -f /tmp/ocr_page-*.png 2>/dev/null

    # Converter PDF para imagens (150 DPI para ser mais rápido)
    if ! pdftoppm -r 200 -png "$pdf" "/tmp/ocr_page" 2>/dev/null; then
        echo "   ❌ Falha ao converter para imagem"
        falhados=$((falhados + 1))
        continue
    fi

    # OCR em cada página
    > "$output"
    for page in /tmp/ocr_page-*.png; do
        if [ -f "$page" ]; then
            tesseract "$page" - -l por 2>/dev/null >> "$output"
            echo "" >> "$output"
        fi
    done

    # Limpar
    rm -f /tmp/ocr_page-*.png

    # Verificar tamanho
    tamanho=$(wc -c < "$output" 2>/dev/null || echo 0)
    if [ "$tamanho" -lt 100 ]; then
        echo "   ⚠️  Texto muito curto ($tamanho chars)"
        falhados=$((falhados + 1))
    else
        echo "   ✅ $tamanho chars extraídos"
        processados=$((processados + 1))
    fi
done

echo ""
echo "========================================="
echo "✅ RESUMO DO OCR"
echo "========================================="
echo "✅ Processados: $processados"
echo "⏭️  Ignorados:  $ignorados"
echo "❌ Falhados:   $falhados"
echo "📂 Resultados: $OUTPUT_DIR"
