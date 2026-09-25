package mcp

const oauthLoginPage = `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Connect Visto</title>
<style>body{font:16px system-ui;background:#11141b;color:#f4f4f5;display:grid;place-items:center;min-height:100vh;margin:0}.panel{width:min(440px,calc(100% - 40px));padding:28px;border:1px solid #3b3e46;border-radius:18px;background:#1b1e26}h1{margin:0 0 8px}p{color:#b7bac3;line-height:1.5}label{display:block;margin:16px 0 6px}input[type=text],input[type=password]{box-sizing:border-box;width:100%;padding:12px;border-radius:10px;border:1px solid #4a4d55;background:#11141b;color:#fff}input[type=checkbox]{accent-color:#f5b52e}button{margin-top:22px;padding:12px 18px;border:0;border-radius:10px;background:#f5b52e;color:#1b1e26;font-weight:700;cursor:pointer}.scope{padding:10px 0;border-top:1px solid #393c44}.error{color:#ff9898}</style>
<main class="panel"><h1>Connect Visto</h1><p><strong>{{.ClientName}}</strong> wants permission to:</p>{{if .Error}}<p class="error">{{.Error}}</p>{{end}}
<form method="post" action="/oauth/authorize"><input type="hidden" name="csrf" value="{{.CSRF}}">{{range .Hidden}}<input type="hidden" name="{{.Name}}" value="{{.Value}}">{{end}}
<div class="scope"><label><input type="checkbox" name="scope" value="read" {{if .Read}}checked{{end}}> Read my Visto library, progress, and watch history</label></div>
<div class="scope"><label><input type="checkbox" name="scope" value="write" {{if .Write}}checked{{end}}> Add titles, update lists, mark media watched, and rate it</label></div>
<div class="scope"><label><input type="checkbox" name="scope" value="offline_access" {{if .Offline}}checked{{end}}> Stay connected between visits</label></div>
<label for="email">Email</label><input id="email" name="email" type="email" autocomplete="email" required>
<label for="password">Visto password</label><input id="password" name="password" type="password" autocomplete="current-password" required>
<button name="consent" value="allow">Authorize ChatGPT</button><button formnovalidate name="consent" value="deny" style="background:#383b43;color:#fff;margin-left:8px">Cancel</button></form></main></html>`
