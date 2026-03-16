(function(){const n=document.createElement("link").relList;if(n&&n.supports&&n.supports("modulepreload"))return;for(const o of document.querySelectorAll('link[rel="modulepreload"]'))r(o);new MutationObserver(o=>{for(const u of o)if(u.type==="childList")for(const g of u.addedNodes)g.tagName==="LINK"&&g.rel==="modulepreload"&&r(g)}).observe(document,{childList:!0,subtree:!0});function a(o){const u={};return o.integrity&&(u.integrity=o.integrity),o.referrerPolicy&&(u.referrerPolicy=o.referrerPolicy),o.crossOrigin==="use-credentials"?u.credentials="include":o.crossOrigin==="anonymous"?u.credentials="omit":u.credentials="same-origin",u}function r(o){if(o.ep)return;o.ep=!0;const u=a(o);fetch(o.href,u)}})();function i(){const e=window.go?.main?.App;if(!e)throw new Error("Wails bindings are not available. Run the project with `wails dev` or `wails build`.");return e}const c={bootstrap:()=>i().Bootstrap(),refreshState:()=>i().RefreshState(),saveSettings:e=>i().SaveSettings(e),saveProject:e=>i().SaveProject(e),deleteProject:e=>i().DeleteProject(e),shareProject:e=>i().ShareProject(e),shareProjectWithRandomURL:e=>i().ShareProjectWithRandomURL(e),startQuickTunnel:e=>i().StartQuickTunnel(e),startTunnel:()=>i().StartTunnel(),stopTunnel:()=>i().StopTunnel(),createTunnel:()=>i().CreateTunnel(),runNpmBuild:e=>i().RunNpmBuild(e),openPublicURL:e=>i().OpenPublicURL(e),openConfigFile:()=>i().OpenConfigFile(),openSettingsFile:()=>i().OpenSettingsFile(),browseProjectFolder:e=>i().BrowseProjectFolder(e),testProject:e=>i().TestProject(e)},j=()=>({id:"",displayName:"",localHost:"",subdomain:"",publicURL:"",projectPath:"",shareMode:"stable"}),t={appState:null,selectedProjectId:null,activeTab:"overview",projectMenuOpen:!1,editorOpen:!1,editorMode:"create",editorProject:j(),notice:null,busy:null},y=document.querySelector("#app");if(!y)throw new Error("App root not found");const v=y;function s(e){return e.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;")}function f(e,n){return e.publicURL?e.publicURL:e.subdomain&&e.shareMode!=="quick"?`https://${e.subdomain}.${n}`:""}function b(){const e=t.appState?.settings.projects??[];return e.length===0?null:t.selectedProjectId?e.find(n=>n.id===t.selectedProjectId)??e[0]:e[0]}function p(e,n){t.notice={tone:e,message:n},m()}function h(e){t.busy=e,m()}function w(e){switch(e){case"random-domain":return"Random domain";case"quick":return"Quick tunnel";default:return"Stable"}}function P(e){return e.running?"success":e.lastError?"error":"idle"}function S(e){return e.slice(-12).reverse().map(n=>`
        <div class="log-entry log-${s(n.level)}">
          <span>${s(new Date(n.timestamp).toLocaleTimeString())}</span>
          <strong>${s(n.source)}</strong>
          <p>${s(n.message)}</p>
        </div>
      `).join("")}function $(e){return e.settings.projects.map(n=>{const a=b()?.id===n.id,r=f(n,e.settings.defaultDomain);return`
        <button type="button" class="project-row ${a?"selected":""}" data-action="select-project" data-id="${s(n.id)}">
          <div class="project-row-top">
            <div>
              <strong>${s(n.displayName)}</strong>
              <span>${s(n.localHost)}</span>
            </div>
            <span class="mini-pill">${s(w(n.shareMode))}</span>
          </div>
          <div class="project-row-bottom">
            <span>${s(n.subdomain||"dynamic")}</span>
            <span>${s(r||"not shared yet")}</span>
          </div>
        </button>
      `}).join("")}function m(){if(!t.appState){v.innerHTML=`
      <main class="shell loading-state">
        <section class="hero-card">
          <h1>Cloudflare Tunnel Manager</h1>
          <p>Loading backend state...</p>
        </section>
      </main>
    `;return}const e=t.appState,n=b();t.selectedProjectId=n?.id??null;const a=e.status,r=a.activeUrl||a.quickUrl||n?.publicURL||"",o=n?f(n,e.settings.defaultDomain):"";v.innerHTML=`
    <main class="shell">
      <section class="masthead">
        <div class="masthead-copy">
          <p class="eyebrow">Cloudflare Tunnel Console</p>
          <h1>Laravel Herd sharing without terminal drift</h1>
          <p class="hero-copy">Manage named tunnel ingress, random share URLs, host-header routing, and local build commands from one Windows desktop panel.</p>
        </div>
        <div class="masthead-status tone-${P(a)}">
          <span class="status-dot"></span>
          <div>
            <strong>${s(a.running?"Tunnel running":"Tunnel stopped")}</strong>
            <p>${s(a.mode||"named")} mode • ${s(e.settings.tunnelName)}</p>
          </div>
        </div>
      </section>

      ${t.notice?`<section class="notice notice-${t.notice.tone}">${s(t.notice.message)}</section>`:""}

      <section class="command-deck">
        <article class="command-bar">
          <div class="command-group">
            <span class="deck-label">Tunnel</span>
            <strong>${s(e.settings.tunnelName)}</strong>
            <span>${s(a.tunnelId||"UUID pending")}</span>
          </div>
          <div class="command-group">
            <span class="deck-label">Cloudflared</span>
            <strong>${s(e.cloudflaredPath||"Not detected")}</strong>
            <span>${s(a.pid?`PID ${a.pid}`:"Process idle")}</span>
          </div>
          <div class="command-group">
            <span class="deck-label">Active URL</span>
            <strong>${s(r||"No public route active")}</strong>
            <span>${s(a.running?"Live session":"Waiting")}</span>
          </div>
          <div class="action-row compact">
            <button type="button" data-action="start-tunnel">Start Tunnel</button>
            <button type="button" class="secondary" data-action="stop-tunnel">Stop</button>
            <button type="button" class="secondary" data-action="create-tunnel">Create / Reuse</button>
            <button type="button" class="secondary" data-action="refresh">Refresh</button>
          </div>
        </article>
      </section>

      <section class="workspace">
        <aside class="panel sidebar-panel">
          <div class="panel-header sticky-header">
            <div>
              <p class="eyebrow">Project presets</p>
              <h2>Laravel Herd apps</h2>
            </div>
            <button type="button" data-action="new-project">Add Project</button>
          </div>
          <div class="sidebar-summary">
            <div class="summary-block">
              <span class="summary-label">Saved projects</span>
              <strong>${s(String(e.settings.projects.length))}</strong>
            </div>
            <div class="summary-block">
              <span class="summary-label">Default domain</span>
              <strong>${s(e.settings.defaultDomain)}</strong>
            </div>
          </div>
          <div class="project-list">${$(e)}</div>
        </aside>

        <section class="content-column">
          <div class="tabs-bar">
            <button type="button" class="tab-button ${t.activeTab==="overview"?"active":""}" data-action="tab-overview">Overview</button>
            <button type="button" class="tab-button ${t.activeTab==="settings"?"active":""}" data-action="tab-settings">Settings</button>
            <button type="button" class="tab-button ${t.activeTab==="logs"?"active":""}" data-action="tab-logs">Logs</button>
          </div>

          ${t.activeTab==="overview"?`
                <article class="panel selected-panel">
                  <div class="panel-header">
                    <div>
                      <p class="eyebrow">Selected project</p>
                      <h2>${s(n?.displayName||"No project selected")}</h2>
                    </div>
                    <div class="selected-header-actions">
                      <span class="pill pill-outline">${s(n?w(n.shareMode):"n/a")}</span>
                      ${n?`
                            <div class="dropdown">
                              <button type="button" class="secondary menu-button" data-action="toggle-project-menu">Manage</button>
                              ${t.projectMenuOpen?`
                                    <div class="dropdown-menu">
                                      <button type="button" class="dropdown-item" data-action="edit-project">Edit project</button>
                                      <button type="button" class="dropdown-item danger-item" data-action="delete-project">Delete project</button>
                                    </div>
                                  `:""}
                            </div>
                          `:""}
                    </div>
                  </div>

                  ${n?`
                        <div class="hero-project">
                          <div class="hero-project-main">
                            <strong>${s(n.localHost)}</strong>
                            <p>${s(o||"No public URL assigned yet")}</p>
                          </div>
                          <div class="host-tags">
                            <span>${s(n.subdomain||"dynamic subdomain")}</span>
                            <span>${s(n.projectPath)}</span>
                          </div>
                        </div>

                        <div class="metric-grid">
                          <div class="metric-card">
                            <span class="summary-label">Origin service</span>
                            <strong>${s(e.settings.defaultServiceURL)}</strong>
                            <p>Requests are forwarded here with the Herd host header override.</p>
                          </div>
                          <div class="metric-card">
                            <span class="summary-label">Host header</span>
                            <strong>${s(n.localHost)}</strong>
                            <p>Applied via <code>originRequest.httpHostHeader</code>.</p>
                          </div>
                          <div class="metric-card">
                            <span class="summary-label">Connected routes</span>
                            <strong>${s(String(a.activeHostnames.length))}</strong>
                            <p>${s(a.activeHostnames.join(", ")||"No ingress hostnames currently loaded.")}</p>
                          </div>
                        </div>

                        <div class="action-cluster">
                          <div class="cluster-block">
                            <span class="cluster-title">Sharing</span>
                            <div class="action-row">
                              <button type="button" data-action="share-project">Share Selected Project</button>
                              <button type="button" class="secondary" data-action="share-random">Random URL</button>
                              <button type="button" class="secondary" data-action="share-quick">Quick Tunnel</button>
                              <button type="button" class="secondary" data-action="open-url">Open URL</button>
                              <button type="button" class="secondary" data-action="copy-url">Copy URL</button>
                            </div>
                          </div>
                          <div class="cluster-block">
                            <span class="cluster-title">Project tools</span>
                            <div class="action-row">
                              <button type="button" class="secondary" data-action="npm-build">Run npm build</button>
                              <button type="button" class="secondary" data-action="test-project">Test local URL</button>
                            </div>
                          </div>
                        </div>
                      `:'<p class="empty-copy">Create or select a project to start sharing.</p>'}
                </article>

                <article class="panel compact-panel">
                  <div class="panel-header">
                    <div>
                      <p class="eyebrow">Tunnel context</p>
                      <h2>Routes and runtime</h2>
                    </div>
                    <span class="pill ${a.running?"pill-success":"pill-muted"}">${s(a.running?"running":"stopped")}</span>
                  </div>
                  <div class="status-grid two-column">
                    <div><label>Config file</label><strong>${s(e.configPath)}</strong></div>
                    <div><label>Cloudflared path</label><strong>${s(e.cloudflaredPath||"not detected")}</strong></div>
                  </div>
                  <div class="status-list">
                    <label>Loaded hostnames</label>
                    <div class="host-tags">
                      ${a.activeHostnames.length?a.activeHostnames.map(u=>`<span>${s(u)}</span>`).join(""):"<span>none</span>"}
                    </div>
                  </div>
                  <div class="action-row">
                    <button type="button" class="secondary" data-action="open-config">Open Config</button>
                    <button type="button" class="secondary" data-action="open-settings">Open Settings</button>
                  </div>
                  ${a.lastError?`<p class="error-copy">${s(a.lastError)}</p>`:""}
                </article>
              `:t.activeTab==="settings"?`
                  <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Defaults</p>
                        <h2>App settings</h2>
                      </div>
                    </div>
                    <form id="settings-form" class="form-grid">
                      <label>Default domain<input name="defaultDomain" value="${s(e.settings.defaultDomain)}" /></label>
                      <label>Tunnel name<input name="tunnelName" value="${s(e.settings.tunnelName)}" /></label>
                      <label>Cloudflared path<input name="cloudflaredPath" value="${s(e.settings.cloudflaredPath)}" placeholder="Leave blank to use PATH" /></label>
                      <label>Local service URL<input name="defaultServiceURL" value="${s(e.settings.defaultServiceURL)}" /></label>
                      <div class="action-row wide"><button type="submit">Save Settings</button></div>
                    </form>
                  </article>
                `:`
                  <section class="panel logs-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Logs</p>
                        <h2>cloudflared and npm output</h2>
                      </div>
                      <span class="pill pill-outline">${s(`${e.status.lastLogs.length} entries`)}</span>
                    </div>
                    <div class="log-stream">${S(e.status.lastLogs)}</div>
                  </section>
                `}

          ${t.editorOpen?`
              <section class="panel editor-panel">
                <div class="panel-header">
                  <div>
                    <p class="eyebrow">${t.editorMode==="create"?"New project":"Edit project"}</p>
                    <h2>${s(t.editorProject.displayName||"Project preset")}</h2>
                  </div>
                  <button type="button" class="secondary" data-action="close-editor">Close</button>
                </div>
                <form id="project-form" class="form-grid editor-grid">
                  <input type="hidden" name="id" value="${s(t.editorProject.id)}" />
                  <label>Display name<input name="displayName" value="${s(t.editorProject.displayName)}" /></label>
                  <label>Local host<input name="localHost" value="${s(t.editorProject.localHost)}" placeholder="hr-system.test" /></label>
                  <label>
                    Project folder
                    <div class="folder-picker">
                      <input name="projectPath" value="${s(t.editorProject.projectPath)}" placeholder="D:\\code\\hr-system" />
                      <button type="button" class="secondary browse-button" data-action="browse-project-folder">Browse</button>
                    </div>
                  </label>
                  <label>Stable subdomain<input name="subdomain" value="${s(t.editorProject.subdomain)}" placeholder="app" /></label>
                  <label>
                    Share mode
                    <select name="shareMode">
                      <option value="stable" ${t.editorProject.shareMode==="stable"?"selected":""}>Stable hostname</option>
                      <option value="random-domain" ${t.editorProject.shareMode==="random-domain"?"selected":""}>Random under my domain</option>
                      <option value="quick" ${t.editorProject.shareMode==="quick"?"selected":""}>Quick tunnel</option>
                    </select>
                  </label>
                  <div class="action-row wide"><button type="submit">${t.editorMode==="create"?"Save Project":"Update Project"}</button></div>
                </form>
              </section>
            `:""}
        </section>
      </section>

      <footer class="footer">
        <span>Settings file: ${s(e.settingsPath)}</span>
        <span>${s(t.busy||"Ready")}</span>
      </footer>
    </main>
  `,R()}function d(e,n){return e.elements.namedItem(n)?.value?.trim()??""}function L(){const e=v.querySelector("#project-form");e&&(t.editorProject={id:d(e,"id"),displayName:d(e,"displayName"),localHost:d(e,"localHost"),projectPath:d(e,"projectPath"),subdomain:d(e,"subdomain"),publicURL:t.editorProject.publicURL,shareMode:d(e,"shareMode")||"stable"})}async function l(e,n){try{h(e);const a=await n();return t.notice=null,a}catch(a){const r=a instanceof Error?a.message:String(a);p("error",r);return}finally{h(null)}}function R(){const e=v.querySelector("#settings-form");e?.addEventListener("submit",async a=>{a.preventDefault();const r={...t.appState.settings,defaultDomain:d(e,"defaultDomain"),tunnelName:d(e,"tunnelName"),cloudflaredPath:d(e,"cloudflaredPath"),defaultServiceURL:d(e,"defaultServiceURL")},o=await l("Saving settings...",()=>c.saveSettings(r));o&&(t.appState=o,p("success","Settings saved"))});const n=v.querySelector("#project-form");n?.addEventListener("submit",async a=>{a.preventDefault();const r={id:d(n,"id"),displayName:d(n,"displayName"),localHost:d(n,"localHost"),projectPath:d(n,"projectPath"),subdomain:d(n,"subdomain"),publicURL:t.editorProject.publicURL,shareMode:d(n,"shareMode")||"stable"},o=await l("Saving project...",()=>c.saveProject(r));o&&(t.appState=o,t.selectedProjectId=r.id||(o.settings.projects[o.settings.projects.length-1]?.id??null),t.editorOpen=!1,p("success","Project preset saved"))})}async function T(e,n){switch(e){case"tab-overview":t.activeTab="overview",t.projectMenuOpen=!1,m();return;case"tab-settings":t.activeTab="settings",t.projectMenuOpen=!1,m();return;case"tab-logs":t.activeTab="logs",t.projectMenuOpen=!1,m();return;case"toggle-project-menu":t.projectMenuOpen=!t.projectMenuOpen,m();return;case"select-project":t.selectedProjectId=n??null,t.projectMenuOpen=!1,m();return;case"new-project":t.editorOpen=!0,t.editorMode="create",t.editorProject=j(),m();return;case"edit-project":b()&&(t.editorOpen=!0,t.editorMode="edit",t.editorProject={...b()},t.projectMenuOpen=!1,m());return;case"close-editor":t.editorOpen=!1,m();return;case"browse-project-folder":{L();const a=await l("Opening folder picker...",()=>c.browseProjectFolder(t.editorProject.projectPath));typeof a=="string"&&a&&(t.editorProject.projectPath=a,m());return}case"copy-url":{t.projectMenuOpen=!1;const a=b();if(!a||!t.appState)return;const r=f(a,t.appState.settings.defaultDomain);if(!r){p("error","No public URL is available for the selected project");return}await navigator.clipboard.writeText(r),p("success",`Copied ${r}`);return}}if(!n&&["share-project","share-random","share-quick","open-url","npm-build","test-project","delete-project"].includes(e)){p("error","Select a project first");return}switch(e){case"start-tunnel":{const a=await l("Starting tunnel...",()=>c.startTunnel());a&&(t.appState=a,p("success","Named tunnel started"));return}case"stop-tunnel":{const a=await l("Stopping tunnel...",()=>c.stopTunnel());a&&(t.appState=a,p("success","Tunnel stopped"));return}case"create-tunnel":{const a=await l("Ensuring named tunnel...",()=>c.createTunnel());a&&(t.appState=a,p("success","Named tunnel is ready"));return}case"open-config":await l("Opening config file...",()=>c.openConfigFile());return;case"open-settings":await l("Opening settings file...",()=>c.openSettingsFile());return;case"refresh":{const a=await l("Refreshing state...",()=>c.refreshState());a&&(t.appState=a);return}case"share-project":{const a=await l("Sharing project...",()=>c.shareProject(n));a&&(t.appState=a,p("success","Project shared through named tunnel"));return}case"share-random":{const a=await l("Generating random hostname...",()=>c.shareProjectWithRandomURL(n));a&&(t.appState=a,p("success","Random domain share is active"));return}case"share-quick":{const a=await l("Starting quick tunnel...",()=>c.startQuickTunnel(n));a&&(t.appState=a,p("success","Quick tunnel started"));return}case"open-url":await l("Opening public URL...",()=>c.openPublicURL(n));return;case"npm-build":{await l("Running npm build...",()=>c.runNpmBuild(n))!==void 0&&p("success","npm build started. Watch the log stream for progress");return}case"test-project":{const a=await l("Testing local URL...",()=>c.testProject(n));typeof a=="string"&&p("success",a);return}case"delete-project":if(window.confirm("Delete this project preset?")){const a=await l("Deleting project...",()=>c.deleteProject(n));a&&(t.appState=a,t.selectedProjectId=a.settings.projects[0]?.id??null,t.projectMenuOpen=!1,p("success","Project deleted"))}return}}v.addEventListener("click",e=>{const n=e.target,a=n?.closest(".dropdown");if(t.projectMenuOpen&&!a){t.projectMenuOpen=!1,m();return}const r=n?.closest("[data-action]");if(!r)return;e.preventDefault();const o=r.dataset.action,u=r.dataset.id??b()?.id??null;o&&T(o,u)});async function O(){const e=await l("Loading app state...",()=>c.bootstrap());e&&(t.appState=e,t.selectedProjectId=e.settings.projects[0]?.id??null,m(),window.runtime?.EventsOn("log",n=>{if(!t.appState)return;const a=n;t.appState.status.lastLogs=[...t.appState.status.lastLogs,a].slice(-300),m()}),window.runtime?.EventsOn("status",n=>{t.appState&&(t.appState.status=n,m())}))}O();
