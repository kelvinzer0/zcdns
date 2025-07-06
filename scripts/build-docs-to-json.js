import fs from 'fs';
import path from 'path';
import matter from 'gray-matter';
import { fileURLToPath } from 'url';
import { marked } from 'marked';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const baseDir = path.join(__dirname, '../content/docs');
const outDir = path.join(__dirname, '../public/docs');
// ✅ [Tambahkan di akhir file scripts/build-docs-to-json.ts]
const index = [];

function walk(localePath, locale) {
    const outLocalePath = path.join(outDir, locale);
    fs.mkdirSync(outLocalePath, { recursive: true });

    function parse(dir) {
        const entries = fs.readdirSync(dir, { withFileTypes: true });
        for (const entry of entries) {
            const fullPath = path.join(dir, entry.name);
            if (entry.isDirectory()) {
                parse(fullPath);
            } else if (entry.name.endsWith('.mdx')) {
                const relPath = path.relative(localePath, fullPath).replace(/\.mdx$/, '');
                const outPath = path.join(outLocalePath, `${relPath}.json`);
                fs.mkdirSync(path.dirname(outPath), { recursive: true });

                const raw = fs.readFileSync(fullPath, 'utf8');
                const { data, content } = matter(raw);
                const htmlContent = marked.parse(content); // Convert Markdown to HTML
                const result = { frontMatter: data, content: htmlContent }; // Store HTML content
                fs.writeFileSync(outPath, JSON.stringify(result, null, 2));

                index.push({
                    locale,
                    slug: relPath.replace(/\\/g, '/'), // format slug pakai slash
                });
            }
        }
    }

    parse(localePath);
}

// Jalankan untuk semua locale
for (const locale of fs.readdirSync(baseDir)) {
    const localePath = path.join(baseDir, locale);
    walk(localePath, locale);
}

// ✅ Simpan index ke public/docs-index.json
fs.writeFileSync(
    path.join(__dirname, '../../public/docs-index.json'),
    JSON.stringify(index, null, 2)
);

