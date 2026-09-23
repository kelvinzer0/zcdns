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
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', checkKey);
  } else {
    checkKey();
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
