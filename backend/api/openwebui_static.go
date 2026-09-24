package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// locateOpenWebUIDir resolves the directory containing static OpenWebUI files
func (h *APIHandler) locateOpenWebUIDir() string {
	candidates := []string{
		filepath.Join(h.cfg.StaticDir, "openwebui"),
		"/opt/zcdns/dist/openwebui",
		"./dist/openwebui",
		"../dist/openwebui",
	}
	for _, dir := range candidates {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
				return dir
			}
		}
	}
	return ""
}

const authBootstrapScript = `<script>
(function() {
  var loc = localStorage.getItem('locale');
  if (!loc || loc === 'en' || loc === 'null' || loc === 'undefined') {
    localStorage.setItem('locale', 'en-US');
  }
  var p = new URLSearchParams(window.location.search);
  var k = p.get('key');
  if (k) {
    localStorage.setItem('token', k);
    localStorage.setItem('zcdns_key', k);
    p.delete('key');
    var s = p.toString() ? ('?' + p.toString()) : '';
    window.history.replaceState({}, document.title, window.location.pathname + s);
  }
  function checkKey() {
    var cur = localStorage.getItem('token') || localStorage.getItem('zcdns_key');
    if (!cur || cur === 'undefined' || cur === 'null' || cur.trim() === '') {
      showModal();
    }
  }
  function showModal() {
    if (document.getElementById('zcdns-key-modal')) return;
    var o = document.createElement('div');
    o.id = 'zcdns-key-modal';
    o.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,0.85);z-index:9999999;display:flex;align-items:center;justify-content:center;padding:16px;font-family:system-ui,-apple-system,sans-serif;';
    var b = document.createElement('div');
    b.style.cssText = 'background:#18181b;color:#f4f4f5;border:1px solid #27272a;max-width:400px;width:100%;padding:24px;box-shadow:0 25px 50px -12px rgba(0,0,0,0.5);';
    b.innerHTML = '<div style="margin-bottom:16px;"><h2 style="margin:0 0 4px 0;font-size:16px;font-weight:700;color:#fff;">AI Router Authentication</h2><p style="margin:0;font-size:12px;color:#a1a1aa;">Enter your client API key to connect to this router.</p></div><div style="margin-bottom:16px;"><label style="display:block;font-size:11px;font-weight:600;color:#d4d4d8;margin-bottom:6px;text-transform:uppercase;">Client API Key</label><input id="zcdns-key-input" type="password" placeholder="zck_..." style="width:100%;box-sizing:border-box;background:#09090b;border:1px solid #27272a;color:#fff;padding:8px 12px;font-size:13px;font-family:monospace;outline:none;" /></div><div style="display:flex;gap:8px;"><button id="zcdns-key-submit" style="flex:1;background:#fff;color:#000;border:none;padding:8px 14px;font-size:12px;font-weight:600;cursor:pointer;">Connect</button></div><div style="margin-top:14px;font-size:11px;color:#71717a;text-align:center;">Manage keys on <a href="https://zcdns.id" target="_blank" style="color:#60a5fa;text-decoration:underline;">ZCDNS Dashboard</a></div>';
    o.appendChild(b);
    document.body.appendChild(o);
    var inp = document.getElementById('zcdns-key-input');
    var btn = document.getElementById('zcdns-key-submit');
    if (inp) inp.focus();
    function submit() {
      var v = (inp && inp.value || '').trim();
      if (!v) { if (inp) inp.style.borderColor = '#ef4444'; return; }
      localStorage.setItem('token', v);
      localStorage.setItem('zcdns_key', v);
      o.remove();
      window.location.reload();
    }
    if (btn) btn.onclick = submit;
    if (inp) inp.onkeydown = function(e) { if (e.key === 'Enter') submit(); };
  }
  function patchConnectionsTab() {
    var tab = document.getElementById('tab-connections');
    if (!tab || tab.getAttribute('data-zcdns-patched') === 'true') return;
    tab.setAttribute('data-zcdns-patched', 'true');
    var lang = (localStorage.getItem('locale') || navigator.language || 'en').toLowerCase().startsWith('id') ? 'id' : 'en';
    var dashUrl = 'https://zcdns.id/' + lang + '/dashboard';
    var isId = (lang === 'id');
    var title = isId ? 'Kelola Koneksi di ZCDNS Dashboard' : 'Manage Connections in ZCDNS Dashboard';
    var desc = isId
      ? 'Koneksi model AI, endpoint API, dan routing dikelola secara terpusat melalui <strong>ZCDNS Dashboard</strong>. Silakan atur endpoint dan provider AI Anda lewat dashboard.'
      : 'AI model connections, API endpoints, and routing are centrally managed via the <strong>ZCDNS Dashboard</strong>. Please configure your endpoints and AI providers through the dashboard.';
    var btnText = isId ? 'Buka Dashboard ZCDNS' : 'Open ZCDNS Dashboard';
    var noteTitle = isId ? 'Informasi AI Router & Endpoint' : 'AI Router & Endpoint Information';
    var noteDesc = isId
      ? 'Semua request chat ke model yang terdaftar di workspace ini secara otomatis diarahkan melalui ZCDNS AI Router proxy. Anda tidak perlu menambahkan custom endpoint secara manual di browser.'
      : 'All chat completions for models registered in this workspace are automatically routed through the ZCDNS AI Router proxy. You do not need to configure custom endpoints manually in the browser.';

    tab.innerHTML = '<h2 class="text-sm font-medium text-gray-900 dark:text-white mb-4">' + (isId ? 'Koneksi' : 'Connections') + '</h2>' +
      '<div class="flex flex-1 min-h-0 flex-col overflow-y-auto pr-1.5 space-y-4">' +
        '<div style="border-radius:1rem;border:1px solid rgba(59,130,246,0.25);background:rgba(59,130,246,0.06);padding:1.25rem;">' +
          '<div style="display:flex;align-items:flex-start;gap:0.875rem;">' +
            '<div style="width:2.25rem;height:2.25rem;display:flex;align-items:center;justify-content:center;border-radius:0.75rem;background:rgba(59,130,246,0.15);color:#3b82f6;flex-shrink:0;">' +
              '<svg style="width:1.25rem;height:1.25rem;" viewBox="0 0 24 24" fill="currentColor"><path fill-rule="evenodd" d="M19.952 1.651a.75.75 0 0 1 .298.599V16.303a3 3 0 0 1-.879 2.121l-4.5 4.5a3 3 0 0 1-2.121.879H4.5a3 3 0 0 1-3-3V4.5a3 3 0 0 1 3-3h15.452ZM18.75 3.75H4.5a1.5 1.5 0 0 0-1.5 1.5v13.5a1.5 1.5 0 0 0 1.5 1.5h7.5V17.25a3 3 0 0 1 3-3h3.75V3.75Zm-2.25 15.69V16.5a1.5 1.5 0 0 0-1.5-1.5h-2.94l4.44 4.44ZM9 7.5a.75.75 0 0 1 .75.75v3a.75.75 0 0 1-1.5 0v-3A.75.75 0 0 1 9 7.5Zm6 0a.75.75 0 0 1 .75.75v3a.75.75 0 0 1-1.5 0v-3A.75.75 0 0 1 15 7.5Z" clip-rule="evenodd" /></svg>' +
            '</div>' +
            '<div style="flex:1;min-width:0;">' +
              '<h3 style="font-size:0.875rem;font-weight:600;margin:0 0 0.375rem 0;color:inherit;">' + title + '</h3>' +
              '<p style="font-size:0.75rem;line-height:1.4;margin:0 0 1rem 0;color:#9ca3af;">' + desc + '</p>' +
              '<div style="display:flex;flex-wrap:wrap;align-items:center;gap:0.75rem;">' +
                '<a href="' + dashUrl + '" target="_blank" rel="noopener noreferrer" style="display:inline-flex;align-items:center;gap:0.5rem;border-radius:0.75rem;background:#2563eb;padding:0.5rem 1rem;font-size:0.75rem;font-weight:600;color:#fff;text-decoration:none;">' +
                  btnText +
                  '<svg style="width:0.875rem;height:0.875rem;" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M4.25 5.5a.75.75 0 0 0-.75.75v8.5c0 .414.336.75.75.75h8.5a.75.75 0 0 0 .75-.75v-4a.75.75 0 0 1 1.5 0v4A2.25 2.25 0 0 1 12.75 17h-8.5A2.25 2.25 0 0 1 2 14.75v-8.5A2.25 2.25 0 0 1 4.25 4h4a.75.75 0 0 1 0 1.5h-4Z" clip-rule="evenodd" /><path fill-rule="evenodd" d="M6.194 12.753a.75.75 0 0 0 1.06.053L16.5 4.44v2.81a.75.75 0 0 0 1.5 0v-4.5a.75.75 0 0 0-.75-.75h-4.5a.75.75 0 0 0 0 1.5h2.553l-9.056 8.194a.75.75 0 0 0-.053 1.06Z" clip-rule="evenodd" /></svg>' +
                '</a>' +
                '<span style="font-size:0.75rem;font-family:monospace;color:#6b7280;user-select:all;">' + dashUrl + '</span>' +
              '</div>' +
            '</div>' +
          '</div>' +
        '</div>' +
        '<div style="border-radius:0.75rem;border:1px solid rgba(255,255,255,0.06);background:rgba(255,255,255,0.03);padding:1rem;font-size:0.75rem;color:#9ca3af;">' +
          '<div style="font-weight:600;margin-bottom:0.375rem;color:#d1d5db;">' + noteTitle + '</div>' +
          '<p style="margin:0;line-height:1.4;">' + noteDesc + '</p>' +
        '</div>' +
      '</div>';
  }
  var obs = new MutationObserver(function() {
    patchConnectionsTab();
  });
  obs.observe(document.documentElement, { childList: true, subtree: true });

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function() { checkKey(); patchConnectionsTab(); });
  } else {
    checkKey();
    patchConnectionsTab();
  }
})();
</script>`

// handleOpenWebUIStatic serves static assets or index.html for OpenWebUI SPA
func (h *APIHandler) handleOpenWebUIStatic(w http.ResponseWriter, r *http.Request) {
	if setOWUCors(w, r) {
		return
	}

	owuDir := h.locateOpenWebUIDir()
	if owuDir == "" {
		http.Error(w, "OpenWebUI frontend assets not installed yet", http.StatusServiceUnavailable)
		return
	}

	cleanPath := filepath.Clean(r.URL.Path)
	target := filepath.Join(owuDir, cleanPath)
	if stat, err := os.Stat(target); err == nil && !stat.IsDir() {
		http.ServeFile(w, r, target)
		return
	}

	// SPA fallback: inject auth bootstrap script dynamically into index.html
	indexPath := filepath.Join(owuDir, "index.html")
	htmlBytes, err := os.ReadFile(indexPath)
	if err != nil {
		http.ServeFile(w, r, indexPath)
		return
	}

	htmlStr := string(htmlBytes)
	if strings.Contains(htmlStr, "<head>") {
		htmlStr = strings.Replace(htmlStr, "<head>", "<head>"+authBootstrapScript, 1)
	} else {
		htmlStr = authBootstrapScript + htmlStr
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlStr))
}
