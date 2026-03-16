(function(){const a=document.createElement("link").relList;if(a&&a.supports&&a.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))r(s);new MutationObserver(s=>{for(const p of s)if(p.type==="childList")for(const g of p.addedNodes)g.tagName==="LINK"&&g.rel==="modulepreload"&&r(g)}).observe(document,{childList:!0,subtree:!0});function t(s){const p={};return s.integrity&&(p.integrity=s.integrity),s.referrerPolicy&&(p.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?p.credentials="include":s.crossOrigin==="anonymous"?p.credentials="omit":p.credentials="same-origin",p}function r(s){if(s.ep)return;s.ep=!0;const p=t(s);fetch(s.href,p)}})();function i(){const e=window.go?.main?.App;if(!e)throw new Error("Wails bindings are not available. Run the project with `wails dev` or `wails build`.");return e}const c={bootstrap:()=>i().Bootstrap(),refreshState:()=>i().RefreshState(),saveSettings:e=>i().SaveSettings(e),saveProject:e=>i().SaveProject(e),deleteProject:e=>i().DeleteProject(e),shareProject:e=>i().ShareProject(e),shareProjectWithRandomURL:e=>i().ShareProjectWithRandomURL(e),startQuickTunnel:e=>i().StartQuickTunnel(e),startTunnel:()=>i().StartTunnel(),stopTunnel:()=>i().StopTunnel(),createTunnel:()=>i().CreateTunnel(),runNpmBuild:e=>i().RunNpmBuild(e),openPublicURL:e=>i().OpenPublicURL(e),openConfigFile:()=>i().OpenConfigFile(),openSettingsFile:()=>i().OpenSettingsFile(),browseProjectFolder:e=>i().BrowseProjectFolder(e),testProject:e=>i().TestProject(e)},j=()=>({id:"",displayName:"",localHost:"",subdomain:"",publicURL:"",projectPath:"",shareMode:"stable"}),n={appState:null,selectedProjectId:null,editorOpen:!1,editorMode:"create",editorProject:j(),notice:null,busy:null},y=document.querySelector("#app");if(!y)throw new Error("App root not found");const f=y;function o(e){return e.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;")}function h(e,a){return e.publicURL?e.publicURL:e.subdomain&&e.shareMode!=="quick"?`https://${e.subdomain}.${a}`:""}function m(){const e=n.appState?.settings.projects??[];return e.length===0?null:n.selectedProjectId?e.find(a=>a.id===n.selectedProjectId)??e[0]:e[0]}function u(e,a){n.notice={tone:e,message:a},b()}function v(e){n.busy=e,b()}function P(e){switch(e){case"random-domain":return"Random domain";case"quick":return"Quick tunnel";default:return"Stable"}}function S(e){return e.running?"success":e.lastError?"error":"idle"}function w(e){return e.slice(-12).reverse().map(a=>`
        <div class="log-entry log-${o(a.level)}">
          <span>${o(new Date(a.timestamp).toLocaleTimeString())}</span>
          <strong>${o(a.source)}</strong>
          <p>${o(a.message)}</p>
        </div>
      `).join("")}function $(e){return e.settings.projects.map(a=>{const t=m()?.id===a.id,r=h(a,e.settings.defaultDomain);return`
        <button type="button" class="project-row ${t?"selected":""}" data-action="select-project" data-id="${o(a.id)}">
          <div class="project-main">
            <strong>${o(a.displayName)}</strong>
            <span>${o(a.localHost)}</span>
          </div>
          <div class="project-meta">
            <span>${o(P(a.shareMode))}</span>
            <span>${o(a.subdomain||"dynamic")}</span>
            <span>${o(r||"not shared yet")}</span>
          </div>
        </button>
      `}).join("")}function b(){if(!n.appState){f.innerHTML=`
      <main class="shell loading-state">
        <section class="hero-card">
          <h1>Cloudflare Tunnel Manager</h1>
          <p>Loading backend state...</p>
        </section>
      </main>
    `;return}const e=n.appState,a=m();n.selectedProjectId=a?.id??null;const t=e.status;f.innerHTML=`
    <main class="shell">
      <section class="hero-card">
        <div>
          <p class="eyebrow">Windows desktop MVP</p>
          <h1>Cloudflare Tunnel Manager for Laravel Herd</h1>
          <p class="hero-copy">Manage named tunnel ingress, random sharing, quick tunnel launches, config updates, and local npm builds without hand-editing <code>config.yml</code>.</p>
        </div>
        <div class="hero-status tone-${S(t)}">
          <span class="status-dot"></span>
          <div>
            <strong>${o(t.running?"Tunnel running":"Tunnel stopped")}</strong>
            <p>${o(t.mode||"named")} | ${o(e.settings.tunnelName)}</p>
          </div>
        </div>
      </section>

      ${n.notice?`<section class="notice notice-${n.notice.tone}">${o(n.notice.message)}</section>`:""}

      <section class="grid top-grid">
        <article class="card">
          <div class="card-header">
            <div>
              <p class="eyebrow">Tunnel status</p>
              <h2>${o(e.settings.tunnelName)}</h2>
            </div>
            <span class="pill ${t.running?"pill-success":"pill-muted"}">${o(t.running?"running":"stopped")}</span>
          </div>
          <div class="status-grid">
            <div><label>Tunnel UUID</label><strong>${o(t.tunnelId||"not resolved")}</strong></div>
            <div><label>PID</label><strong>${o(t.pid?String(t.pid):"n/a")}</strong></div>
            <div><label>Cloudflared</label><strong>${o(e.cloudflaredPath||"not detected")}</strong></div>
            <div><label>Config</label><strong>${o(e.configPath)}</strong></div>
          </div>
          <div class="status-list">
            <label>Current connected hostnames</label>
            <div class="host-tags">
              ${t.activeHostnames.length?t.activeHostnames.map(r=>`<span>${o(r)}</span>`).join(""):"<span>none</span>"}
            </div>
          </div>
          <div class="status-list">
            <label>Active public URL</label>
            <strong>${o(t.activeUrl||t.quickUrl||a?.publicURL||"not active")}</strong>
          </div>
          ${t.lastError?`<p class="error-copy">${o(t.lastError)}</p>`:""}
          <div class="action-row">
            <button type="button" data-action="start-tunnel">Start Tunnel</button>
            <button type="button" class="secondary" data-action="stop-tunnel">Stop Tunnel</button>
            <button type="button" class="secondary" data-action="create-tunnel">Create / Reuse Tunnel</button>
            <button type="button" class="secondary" data-action="open-config">Open Config File</button>
            <button type="button" class="secondary" data-action="open-settings">Open Settings File</button>
            <button type="button" class="secondary" data-action="refresh">Refresh</button>
          </div>
        </article>

        <article class="card">
          <div class="card-header">
            <div>
              <p class="eyebrow">Settings</p>
              <h2>Defaults</h2>
            </div>
          </div>
          <form id="settings-form" class="form-grid">
            <label>Default domain<input name="defaultDomain" value="${o(e.settings.defaultDomain)}" /></label>
            <label>Tunnel name<input name="tunnelName" value="${o(e.settings.tunnelName)}" /></label>
            <label>Cloudflared path<input name="cloudflaredPath" value="${o(e.settings.cloudflaredPath)}" placeholder="Leave blank to use PATH" /></label>
            <label>Local service URL<input name="defaultServiceURL" value="${o(e.settings.defaultServiceURL)}" /></label>
            <div class="action-row wide"><button type="submit">Save Settings</button></div>
          </form>
        </article>
      </section>

      <section class="grid main-grid">
        <article class="card project-card">
          <div class="card-header">
            <div>
              <p class="eyebrow">Project presets</p>
              <h2>Laravel Herd apps</h2>
            </div>
            <button type="button" data-action="new-project">Add Project</button>
          </div>
          <div class="project-list">${$(e)}</div>
        </article>

        <article class="card detail-card">
          <div class="card-header">
            <div>
              <p class="eyebrow">Selected project</p>
              <h2>${o(a?.displayName||"No project selected")}</h2>
            </div>
            <span class="pill pill-outline">${o(a?P(a.shareMode):"n/a")}</span>
          </div>
          ${a?`
                <div class="detail-grid">
                  <div><label>Local Herd hostname</label><strong>${o(a.localHost)}</strong></div>
                  <div><label>Subdomain</label><strong>${o(a.subdomain||"dynamic")}</strong></div>
                  <div><label>Project folder</label><strong>${o(a.projectPath)}</strong></div>
                  <div><label>Public URL</label><strong>${o(h(a,e.settings.defaultDomain)||"not shared")}</strong></div>
                </div>
                <div class="action-row">
                  <button type="button" data-action="share-project">Share Selected Project</button>
                  <button type="button" class="secondary" data-action="share-random">Share with Random URL</button>
                  <button type="button" class="secondary" data-action="share-quick">Quick Tunnel</button>
                  <button type="button" class="secondary" data-action="open-url">Open Public URL</button>
                  <button type="button" class="secondary" data-action="copy-url">Copy URL</button>
                </div>
                <div class="action-row">
                  <button type="button" class="secondary" data-action="npm-build">Run npm build</button>
                  <button type="button" class="secondary" data-action="test-project">Test local URL</button>
                  <button type="button" class="secondary" data-action="edit-project">Edit Project</button>
                  <button type="button" class="danger" data-action="delete-project">Delete Project</button>
                </div>
              `:'<p class="empty-copy">Create or select a project to start sharing.</p>'}
        </article>
      </section>

      ${n.editorOpen?`
          <section class="card editor-card">
            <div class="card-header">
              <div>
                <p class="eyebrow">${n.editorMode==="create"?"New project":"Edit project"}</p>
                <h2>${o(n.editorProject.displayName||"Project preset")}</h2>
              </div>
              <button type="button" class="secondary" data-action="close-editor">Close</button>
            </div>
            <form id="project-form" class="form-grid editor-grid">
              <input type="hidden" name="id" value="${o(n.editorProject.id)}" />
              <label>Display name<input name="displayName" value="${o(n.editorProject.displayName)}" /></label>
              <label>Local host<input name="localHost" value="${o(n.editorProject.localHost)}" placeholder="hr-system.test" /></label>
              <label>
                Project folder
                <div class="folder-picker">
                  <input name="projectPath" value="${o(n.editorProject.projectPath)}" placeholder="D:\\code\\hr-system" />
                  <button type="button" class="secondary browse-button" data-action="browse-project-folder">Browse</button>
                </div>
              </label>
              <label>Stable subdomain<input name="subdomain" value="${o(n.editorProject.subdomain)}" placeholder="app" /></label>
              <label>
                Share mode
                <select name="shareMode">
                  <option value="stable" ${n.editorProject.shareMode==="stable"?"selected":""}>Stable hostname</option>
                  <option value="random-domain" ${n.editorProject.shareMode==="random-domain"?"selected":""}>Random under my domain</option>
                  <option value="quick" ${n.editorProject.shareMode==="quick"?"selected":""}>Quick tunnel</option>
                </select>
              </label>
              <div class="action-row wide"><button type="submit">${n.editorMode==="create"?"Save Project":"Update Project"}</button></div>
            </form>
          </section>
        `:""}

      <section class="card log-card">
        <div class="card-header">
          <div>
            <p class="eyebrow">Logs</p>
            <h2>cloudflared and npm output</h2>
          </div>
          <span class="pill pill-outline">${o(`${e.status.lastLogs.length} entries`)}</span>
        </div>
        <div class="log-stream">${w(e.status.lastLogs)}</div>
      </section>

      <footer class="footer">
        <span>Settings file: ${o(e.settingsPath)}</span>
        <span>${o(n.busy||"Ready")}</span>
      </footer>
    </main>
  `,R()}function d(e,a){return e.elements.namedItem(a)?.value?.trim()??""}function L(){const e=f.querySelector("#project-form");e&&(n.editorProject={id:d(e,"id"),displayName:d(e,"displayName"),localHost:d(e,"localHost"),projectPath:d(e,"projectPath"),subdomain:d(e,"subdomain"),publicURL:n.editorProject.publicURL,shareMode:d(e,"shareMode")||"stable"})}async function l(e,a){try{v(e);const t=await a();return n.notice=null,t}catch(t){const r=t instanceof Error?t.message:String(t);u("error",r);return}finally{v(null)}}function R(){const e=f.querySelector("#settings-form");e?.addEventListener("submit",async t=>{t.preventDefault();const r={...n.appState.settings,defaultDomain:d(e,"defaultDomain"),tunnelName:d(e,"tunnelName"),cloudflaredPath:d(e,"cloudflaredPath"),defaultServiceURL:d(e,"defaultServiceURL")},s=await l("Saving settings...",()=>c.saveSettings(r));s&&(n.appState=s,u("success","Settings saved"))});const a=f.querySelector("#project-form");a?.addEventListener("submit",async t=>{t.preventDefault();const r={id:d(a,"id"),displayName:d(a,"displayName"),localHost:d(a,"localHost"),projectPath:d(a,"projectPath"),subdomain:d(a,"subdomain"),publicURL:n.editorProject.publicURL,shareMode:d(a,"shareMode")||"stable"},s=await l("Saving project...",()=>c.saveProject(r));s&&(n.appState=s,n.selectedProjectId=r.id||(s.settings.projects[s.settings.projects.length-1]?.id??null),n.editorOpen=!1,u("success","Project preset saved"))})}async function U(e,a){switch(e){case"select-project":n.selectedProjectId=a??null,b();return;case"new-project":n.editorOpen=!0,n.editorMode="create",n.editorProject=j(),b();return;case"edit-project":m()&&(n.editorOpen=!0,n.editorMode="edit",n.editorProject={...m()},b());return;case"close-editor":n.editorOpen=!1,b();return;case"browse-project-folder":{L();const t=await l("Opening folder picker...",()=>c.browseProjectFolder(n.editorProject.projectPath));typeof t=="string"&&t&&(n.editorProject.projectPath=t,b());return}case"copy-url":{const t=m();if(!t||!n.appState)return;const r=h(t,n.appState.settings.defaultDomain);if(!r){u("error","No public URL is available for the selected project");return}await navigator.clipboard.writeText(r),u("success",`Copied ${r}`);return}}if(!a&&["share-project","share-random","share-quick","open-url","npm-build","test-project","delete-project"].includes(e)){u("error","Select a project first");return}switch(e){case"start-tunnel":{const t=await l("Starting tunnel...",()=>c.startTunnel());t&&(n.appState=t,u("success","Named tunnel started"));return}case"stop-tunnel":{const t=await l("Stopping tunnel...",()=>c.stopTunnel());t&&(n.appState=t,u("success","Tunnel stopped"));return}case"create-tunnel":{const t=await l("Ensuring named tunnel...",()=>c.createTunnel());t&&(n.appState=t,u("success","Named tunnel is ready"));return}case"open-config":await l("Opening config file...",()=>c.openConfigFile());return;case"open-settings":await l("Opening settings file...",()=>c.openSettingsFile());return;case"refresh":{const t=await l("Refreshing state...",()=>c.refreshState());t&&(n.appState=t);return}case"share-project":{const t=await l("Sharing project...",()=>c.shareProject(a));t&&(n.appState=t,u("success","Project shared through named tunnel"));return}case"share-random":{const t=await l("Generating random hostname...",()=>c.shareProjectWithRandomURL(a));t&&(n.appState=t,u("success","Random domain share is active"));return}case"share-quick":{const t=await l("Starting quick tunnel...",()=>c.startQuickTunnel(a));t&&(n.appState=t,u("success","Quick tunnel started"));return}case"open-url":await l("Opening public URL...",()=>c.openPublicURL(a));return;case"npm-build":{await l("Running npm build...",()=>c.runNpmBuild(a))!==void 0&&u("success","npm build started. Watch the log stream for progress");return}case"test-project":{const t=await l("Testing local URL...",()=>c.testProject(a));typeof t=="string"&&u("success",t);return}case"delete-project":if(window.confirm("Delete this project preset?")){const t=await l("Deleting project...",()=>c.deleteProject(a));t&&(n.appState=t,n.selectedProjectId=t.settings.projects[0]?.id??null,u("success","Project deleted"))}return}}f.addEventListener("click",e=>{const t=e.target?.closest("[data-action]");if(!t)return;e.preventDefault();const r=t.dataset.action,s=t.dataset.id??m()?.id??null;r&&U(r,s)});async function T(){const e=await l("Loading app state...",()=>c.bootstrap());e&&(n.appState=e,n.selectedProjectId=e.settings.projects[0]?.id??null,b(),window.runtime?.EventsOn("log",a=>{if(!n.appState)return;const t=a;n.appState.status.lastLogs=[...n.appState.status.lastLogs,t].slice(-300),b()}),window.runtime?.EventsOn("status",a=>{n.appState&&(n.appState.status=a,b())}))}T();
