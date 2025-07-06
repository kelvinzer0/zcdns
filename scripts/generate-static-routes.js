// scripts/generate-static-routes.js
import { copyFile, mkdir, readdir, stat, writeFile } from 'node:fs/promises';

import path from 'node:path';

// --- Konfigurasi ---
const LOCALES = ['en', 'id'];
const DIST_PATH = 'dist';
const DOCS_CONTENT_PATH = 'content/docs';

// PENTING: Ganti dengan domain produksi Anda! Ini wajib untuk sitemap.
const BASE_URL = 'https://www.zcdns.id'; 

// DETEKTOR RUTE:
const COMPONENT_PAGE_ROUTES = [
  '/', 
  '/abuse-report',
  '/grants',
];
// --- Akhir Konfigurasi ---

const projectRoot = process.cwd();
const docsDir = path.join(projectRoot, DOCS_CONTENT_PATH);
const distDir = path.join(projectRoot, DIST_PATH);
const sourceHtmlPath = path.join(distDir, 'index.html');

async function findMdxFiles(dir) {
  try {
    const dirents = await readdir(dir, { withFileTypes: true });
    const files = await Promise.all(dirents.map((dirent) => {
      const res = path.resolve(dir, dirent.name);
      return dirent.isDirectory() ? findMdxFiles(res) : res;
    }));
    return Array.prototype.concat(...files).filter(file => file.endsWith('.mdx'));
  } catch (error) {
    if (error.code === 'ENOENT') {
      console.log(`Info: Direktori ${DOCS_CONTENT_PATH} tidak ditemukan, deteksi rute MDX dilewati.`);
      return [];
    }
    throw error;
  }
}

function mdxFileToRoute(filePath) {
  const relativePath = filePath.replace(docsDir, '').replace(/\\/g, '/');
  const parts = relativePath.split('/');
  const locale = parts[1]; // e.g., 'en' or 'id'
  const docPath = parts.slice(2).join('/').replace(/\.mdx$/, ''); // e.g., 'overview' or 'records/srv'
  return `/${locale}/docs/${docPath}`;
}

async function generateSitemap(routes) {
  if (BASE_URL === 'https://www.yourdomain.com') {
    console.warn('\nPERINGATAN: BASE_URL di scripts/generate-static-routes.js belum diubah.');
    console.warn('Sitemap akan dibuat dengan domain placeholder.');
  }

  console.log('Membuat sitemap.xml dan sitemap.xsl...');

  const today = new Date().toISOString().split('T')[0];

  const sitemapXmlContent = `<?xml version="1.0" encoding="UTF-8"?>\n<?xml-stylesheet type="text/xsl" href="/sitemap.xsl"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${routes
  .map(route => `\n  <url>\n    <loc>${BASE_URL}${route}</loc>\n    <lastmod>${today}</lastmod>\n    <changefreq>monthly</changefreq>\n    <priority>0.8</priority>\n  </url>`)
  .join('')}\n</urlset>`;

  const sitemapXslContent = `<?xml version="1.0" encoding="UTF-8"?>\n<xsl:stylesheet version="2.0" \n                xmlns:html="http://www.w3.org/TR/REC-html40"\n                xmlns:sitemap="http://www.sitemaps.org/schemas/sitemap/0.9"\n                xmlns:xsl="http://www.w3.org/1999/XSL/Transform">\n\t<xsl:output method="html" version="1.0" encoding="UTF-8" indent="yes"/>\n\t<xsl:template match="/">\n\t\t<html xmlns="http://www.w3.org/1999/xhtml">\n\t\t\t<head>\n\t\t\t\t<title>XML Sitemap</title>\n\t\t\t\t<meta http-equiv="Content-Type" content="text/html; charset=utf-8" />\n\t\t\t\t<style type="text/css">\n\t\t\t\t\tbody { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, 'Open Sans', 'Helvetica Neue', sans-serif; color: #333; }\n\t\t\t\t\t#sitemap { max-width: 900px; margin: 2rem auto; }\n\t\t\t\t\ttable { width: 100%; border-collapse: collapse; }\n\t\t\t\t\tth, td { padding: 0.8rem 1rem; text-align: left; border-bottom: 1px solid #ddd; }\n\t\t\t\t\tth { background-color: #f2f2f2; font-weight: 600; }\n          tr:hover { background-color: #f5f5f5; }\n\t\t\t\t\ta { color: #007bff; text-decoration: none; }\n          a:hover { text-decoration: underline; }\n          h1 { font-size: 1.8rem; font-weight: 700; }\n          p { font-size: 0.9rem; color: #666; }\n\t\t\t\t</style>\n\t\t\t</head>\n\t\t\t<body>\n\t\t\t\t<div id="sitemap">\n\t\t\t\t\t<h1>XML Sitemap</h1>\n          <p>Generated <xsl:value-of select="count(sitemap:urlset/sitemap:url)"/> URLs</p>\n\t\t\t\t\t<table>\n\t\t\t\t\t\t<thead>\n\t\t\t\t\t\t\t<tr>\n\t\t\t\t\t\t\t\t<th>URL</th>\n\t\t\t\t\t\t\t\t<th>Last Modified</th>\n\t\t\t\t\t\t\t</tr>\n\t\t\t\t\t\t</thead>\n\t\t\t\t\t\t<tbody>\n\t\t\t\t\t\t\t<xsl:for-each select="sitemap:urlset/sitemap:url">\n\t\t\t\t\t\t\t\t<tr>\n\t\t\t\t\t\t\t\t\t<td>\n\t\t\t\t\t\t\t\t\t\t<xsl:variable name="loc">\n\t\t\t\t\t\t\t\t\t\t\t<xsl:value-of select="sitemap:loc"/>\n\t\t\t\t\t\t\t\t\t\t</xsl:variable>\n\t\t\t\t\t\t\t\t\t\t<a href="{$loc}">\n\t\t\t\t\t\t\t\t\t\t\t<xsl:value-of select="sitemap:loc"/>\n\t\t\t\t\t\t\t\t\t\t</a>\n\t\t\t\t\t\t\t\t\t</td>\n\t\t\t\t\t\t\t\t\t<td>\n\t\t\t\t\t\t\t\t\t\t<xsl:value-of select="sitemap:lastmod"/>\n\t\t\t\t\t\t\t\t\t</td>\n\t\t\t\t\t\t\t\t</tr>\n\t\t\t\t\t\t\t</xsl:for-each>\n\t\t\t\t\t\t</tbody>\n\t\t\t\t\t</table>\n\t\t\t\t</div>\n\t\t\t</body>\n\t\t</html>\n\t</xsl:template>\n</xsl:stylesheet>`;

  try {
    await writeFile(path.join(distDir, 'sitemap.xml'), sitemapXmlContent);
    await writeFile(path.join(distDir, 'sitemap.xsl'), sitemapXslContent);
    console.log('Sitemap berhasil dibuat: dist/sitemap.xml');
  } catch (error) {
    console.error('Gagal membuat file sitemap:', error);
  }
}

async function generateRoutes() {
  console.log('Memulai pembuatan rute statis...');

  try {
    await stat(sourceHtmlPath);
  } catch {
    console.error(`Error: File sumber 'dist/index.html' tidak ditemukan.`);
    console.error('Pastikan Anda menjalankan "vite build" sebelum skrip ini.');
    process.exit(1);
  }

  const mdxFiles = await findMdxFiles(docsDir);
  const docRoutes = mdxFiles.map(mdxFileToRoute);
  if (mdxFiles.length > 0) {
    console.log(`Detektor MDX: Menemukan ${docRoutes.length} rute dokumentasi.`);
  }

  const componentRoutesWithLocale = COMPONENT_PAGE_ROUTES.flatMap(route =>
    LOCALES.map(locale => route === '/' ? `/${locale}` : `/${locale}${route}`)
  );
  console.log(`Detektor Komponen: Menemukan ${COMPONENT_PAGE_ROUTES.length} rute halaman utama, menghasilkan ${componentRoutesWithLocale.length} rute ber-locale.`);

  const allRoutes = [...new Set(['/', ...docRoutes, ...componentRoutesWithLocale])];
  console.log(`Total ${allRoutes.length} rute unik ditemukan.`);

  let createdCount = 0;
  for (const route of allRoutes) {
    if (route === '/' || !route) continue;
    const cleanRoute = route.startsWith('/') ? route.substring(1) : route;
    const routeDir = path.join(distDir, cleanRoute);
    const targetHtmlPath = path.join(routeDir, 'index.html');
    try {
      await mkdir(routeDir, { recursive: true });
      await copyFile(sourceHtmlPath, targetHtmlPath);
      createdCount++;
    } catch (err) {
      console.error(`Gagal membuat rute untuk ${route}:`, err);
    }
  }
  
  if (createdCount > 0) {
    console.log(`\nBerhasil membuat ${createdCount} file rute statis.`);
  } else {
    console.log('\nTidak ada rute statis baru yang dibuat.');
  }

  // Buat sitemap setelah semua rute dibuat
  await generateSitemap(allRoutes);

  console.log('Proses selesai!');
}

generateRoutes().catch(err => {
  console.error('Terjadi kesalahan fatal saat membuat rute statis:', err);
  process.exit(1);
});