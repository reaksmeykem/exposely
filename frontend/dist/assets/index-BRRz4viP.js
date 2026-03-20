(function(){const a=document.createElement("link").relList;if(a&&a.supports&&a.supports("modulepreload"))return;for(const r of document.querySelectorAll('link[rel="modulepreload"]'))s(r);new MutationObserver(r=>{for(const o of r)if(o.type==="childList")for(const p of o.addedNodes)p.tagName==="LINK"&&p.rel==="modulepreload"&&s(p)}).observe(document,{childList:!0,subtree:!0});function n(r){const o={};return r.integrity&&(o.integrity=r.integrity),r.referrerPolicy&&(o.referrerPolicy=r.referrerPolicy),r.crossOrigin==="use-credentials"?o.credentials="include":r.crossOrigin==="anonymous"?o.credentials="omit":o.credentials="same-origin",o}function s(r){if(r.ep)return;r.ep=!0;const o=n(r);fetch(r.href,o)}})();function v(){const t=window.go?.main?.App;if(!t)throw new Error("Wails bindings are not available. Run the project with `wails dev` or `wails build`.");return t}const c={bootstrap:()=>v().Bootstrap(),refreshState:()=>v().RefreshState(),ensureCloudflared:()=>v().EnsureCloudflared(),saveSettings:t=>v().SaveSettings(t),saveProject:t=>v().SaveProject(t),deleteProject:t=>v().DeleteProject(t),shareProject:t=>v().ShareProject(t),shareProjectWithRandomURL:t=>v().ShareProjectWithRandomURL(t),startQuickTunnel:t=>v().StartQuickTunnel(t),startTunnel:()=>v().StartTunnel(),stopTunnel:()=>v().StopTunnel(),createTunnel:()=>v().CreateTunnel(),activateLicense:t=>v().ActivateLicense(t),clearLicense:()=>v().ClearLicense(),runNpmBuild:t=>v().RunNpmBuild(t),openPublicURL:t=>v().OpenPublicURL(t),openConfigFile:()=>v().OpenConfigFile(),openSettingsFile:()=>v().OpenSettingsFile(),browseProjectFolder:t=>v().BrowseProjectFolder(t),testProject:t=>v().TestProject(t)};function B(){window.runtime.WindowCenter()}function F(t,a){window.runtime.WindowSetSize(t,a)}function _(t,a){window.runtime.WindowSetMinSize(t,a)}function V(){window.runtime.WindowUnmaximise()}function G(){return window.runtime.WindowIsMaximised()}const I=()=>({id:"",displayName:"",localHost:"",subdomain:"",publicURL:"",projectPath:"",shareMode:"quick"}),e={appState:null,selectedProjectId:null,activeProjectId:null,projectUrls:{},copiedProjectId:null,activeTab:"overview",projectMenuOpen:!1,editorOpen:!1,editorMode:"create",editorProject:I(),notice:null,busy:null,licenseDraft:""},C=document.querySelector("#app");if(!C)throw new Error("App root not found");const w=C,z=1040,Q=700,Z=900,K=620;let S=null,y=null;function i(t){return t.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;")}function O(t,a){return t.publicURL?t.publicURL:t.subdomain&&t.shareMode!=="quick"?`https://${t.subdomain}.${a}`:""}function x(t){const a=t.trim().replace(/[/\\]+$/,"");if(!a)return"";const n=a.split(/[/\\]+/),s=n[n.length-1]?.trim()??"";if(!s)return"";const r=s.toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-+|-+$/g,"");return r?`${r}.test`:""}function Y(){return Math.random().toString(36).slice(2,8)}function j(){const t=e.appState?.settings.projects??[];return t.length===0?null:e.selectedProjectId?t.find(a=>a.id===e.selectedProjectId)??t[0]:t[0]}function $(t){for(const a of t.settings.projects)a.shareMode!=="quick"&&a.publicURL?.trim()&&(e.projectUrls[a.id]=a.publicURL.trim())}function U(t){const a=(t.status.activeUrl||t.status.quickUrl||"").trim();if(!t.status.running)return null;if(a){const n=t.settings.projects.find(s=>s.publicURL?.trim()===a);if(n)return n.id}return e.selectedProjectId||t.settings.projects[0]?.id||null}function d(t,a){e.notice={tone:t,message:a},S!==null&&window.clearTimeout(S),S=window.setTimeout(()=>{e.notice=null,S=null,u()},3e3),u()}function L(t){e.busy=t,u()}function J(t){return t.running?"success":t.lastError?"error":"idle"}function X(t){return t.slice(-12).reverse().map(a=>`
        <div class="log-entry log-${i(a.level)}">
          <span>${i(new Date(a.timestamp).toLocaleTimeString())}</span>
          <strong>${i(a.source)}</strong>
          <p>${i(a.message)}</p>
        </div>
      `).join("")}function ee(t){return t.settings.projects.map(a=>{const n=j()?.id===a.id,s=(a.displayName||a.localHost||"P").slice(0,2).toUpperCase(),r=e.activeProjectId===a.id&&t.status.running;return`
        <button type="button" class="project-row ${n?"selected":""}" data-action="select-project" data-id="${i(a.id)}">
          <div class="project-avatar">${i(s)}</div>
          <div class="project-copy">
            <strong>${i(a.displayName)}</strong>
            <span>${i(a.localHost)}</span>
          </div>
          ${r?'<span class="project-running-badge">Running</span>':""}
        </button>
      `}).join("")}function u(){if(!e.appState){w.innerHTML=`
      <main class="shell loading-state">
        <section class="hero-card">
          <h1>Cloudflare Tunnel Manager</h1>
          <p>Loading backend state...</p>
        </section>
      </main>
    `;return}const t=e.appState;$(t);const a=j();e.selectedProjectId=a?.id??null;const n=t.status,s=a?a.shareMode==="quick"?"":e.projectUrls[a.id]||a.publicURL||"":"",o=e.activeProjectId===a?.id&&(n.activeUrl||n.quickUrl)||""||s,p=a?O(a,t.settings.defaultDomain):"",h=t.settings.projects.length>0,g=t.cloudflaredDetected,m=g,b=t.license.valid&&t.license.isAdmin,q=g&&h,M=g?"Installed":"Not installed",A=g?"pill-success":"pill-outline",N=g?"cloudflared is installed and ready for sharing.":"cloudflared is not installed yet. Click Install Share Tool first.",T=m?"Available":"Install required",H=m?"pill-success":"pill-outline",D=m?"Create or reuse a named tunnel.":"Install the Share Tool first before setting up a named tunnel.",W=t.cloudflaredPath||t.managedCloudflaredPath,R=g?h?"":"Create a project first":"";w.innerHTML=`
    <main class="shell">
      <aside class="sidebar">
        <div class="sidebar-header">
          <div class="logo">
            <svg width="32" height="32" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
              <rect width="32" height="32" rx="8" fill="#F48120"/>
              <path d="M16 8L24 16L16 24L8 16L16 8Z" fill="white"/>
            </svg>
            <div>
              <h1>Cloudflare Tunnel</h1>
              <div class="status-indicator tone-${J(n)}">
                <span class="status-dot"></span>
                <span>${i(n.running?"Running":"Stopped")}</span>
              </div>
            </div>
          </div>
        </div>

        <nav class="sidebar-nav">
          <button type="button" class="nav-item ${e.activeTab==="overview"?"active":""}" data-action="tab-overview">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M10 2L2 7L10 12L18 7L10 2Z" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
              <path d="M2 13L10 18L18 13" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
            <span>Overview</span>
          </button>
          ${b?`
            <button type="button" class="nav-item ${e.activeTab==="settings"?"active":""}" data-action="tab-settings">
              <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
                <circle cx="10" cy="10" r="3" stroke="currentColor" stroke-width="2"/>
                <path d="M10 2V4M10 16V18M18 10H16M4 10H2M15.66 4.34L14.24 5.76M5.76 14.24L4.34 15.66M15.66 15.66L14.24 14.24M5.76 5.76L4.34 4.34" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
              <span>Settings</span>
            </button>
          `:""}
          <button type="button" class="nav-item ${e.activeTab==="setup"?"active":""}" data-action="tab-setup">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M10 3L16 6.5V13.5L10 17L4 13.5V6.5L10 3Z" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
              <path d="M10 8V10.5" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              <circle cx="10" cy="13" r="1" fill="currentColor"/>
            </svg>
            <span>Setup</span>
          </button>
        </nav>

        <div class="sidebar-section sidebar-create-section">
          <div class="section-header">
            <h3>Projects</h3>
            <button type="button" class="add-button" data-action="new-project">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg">
                <path d="M8 3V13M3 8H13" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              </svg>
            </button>
          </div>
        </div>

        <div class="sidebar-section sidebar-projects-section">
          <div class="project-list">${ee(t)}</div>
        </div>

        <div class="sidebar-footer">
          <div class="nav-item ${e.activeTab==="logs"?"active":""} logs-nav-item" data-action="tab-logs">
            <span>Logs</span>
          </div>
        </div>
      </aside>

      <section class="main-content">
        ${g?"":`
          <section class="install-banner" data-action="tab-setup">
            <div class="install-banner-copy">
              <span class="install-banner-label">Required setup</span>
              <strong>Install Share Tool first</strong>
              <p>cloudflared is not installed yet. Install it before starting a tunnel or sharing a project.</p>
              <span class="install-banner-path">Install path: ${i(W)}</span>
            </div>
            <div class="install-banner-action">
              <button type="button" class="danger-button" data-action="tab-setup">Open Setup</button>
            </div>
          </section>
        `}

        <header class="content-header">
          <div class="header-info">
            <h2>${e.activeTab==="overview"?"Overview":e.activeTab==="settings"?"Settings":e.activeTab==="setup"?"Setup":"Logs"}</h2>
          </div>
          <div class="header-actions">
            <button type="button" data-action="start-tunnel" ${e.appState?.status.running||!q?"disabled":""}>Start Tunnel</button>
            <button type="button" class="secondary" data-action="stop-tunnel">Stop</button>
            <button type="button" class="secondary" data-action="refresh">Refresh</button>
          </div>
        </header>

        ${R?`<p class="header-hint">${i(R)}</p>`:""}

        ${e.notice?`<section class="toast toast-${e.notice.tone}">${i(e.notice.message)}</section>`:""}

          ${e.editorOpen?`
              <section class="panel editor-panel">
                <div class="panel-header">
                  <div>
                    <p class="eyebrow">${e.editorMode==="create"?"New project":"Edit project"}</p>
                    <h2>${i(e.editorProject.displayName||"Project preset")}</h2>
                  </div>
                  <button type="button" class="secondary" data-action="close-editor">Close</button>
                </div>
                <form id="project-form" class="form-grid editor-grid">
                  <input type="hidden" name="id" value="${i(e.editorProject.id)}" />
                  <label>Display name<input name="displayName" value="${i(e.editorProject.displayName)}" /></label>
                  <label>
                    Project folder
                    <div class="folder-picker">
                      <input name="projectPath" value="${i(e.editorProject.projectPath)}" placeholder="D:\\code\\hr-system" />
                      <button type="button" class="secondary browse-button" data-action="browse-project-folder">Browse</button>
                    </div>
                  </label>
                  <label>Local host<input name="localHost" value="${i(e.editorProject.localHost)}" placeholder="hr-system.test" /></label>
                  <label>
                    Share mode
                    <select name="shareMode">
                      <option value="quick" ${e.editorProject.shareMode==="quick"?"selected":""}>One-click public URL (No Cloudflare Login Required)</option>
                      ${b?`<option value="stable" ${e.editorProject.shareMode==="stable"?"selected":""}>Stable hostname (Admin License Required)</option>`:""}
                      ${b?`<option value="random-domain" ${e.editorProject.shareMode==="random-domain"?"selected":""}>Random under my domain (Admin License Required)</option>`:""}
                    </select>
                  </label>
                  ${b&&e.editorProject.shareMode==="stable"?`
                        <label>
                          Stable subdomain
                          <div class="folder-picker">
                            <input name="subdomain" value="${i(e.editorProject.subdomain)}" placeholder="app" />
                            <button type="button" class="secondary browse-button" data-action="random-subdomain">Random</button>
                          </div>
                        </label>
                      `:""}
                   ${b?e.editorProject.shareMode==="quick"?`
                         <div class="field-hint hint-success">
                           <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                            <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                            <path d="M4.5 7L6 8.5L9.5 5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                          </svg>
                          <span>This mode works without a Cloudflare login.</span>
                        </div>
                      `:"":`
                         <div class="field-hint hint-success">
                           <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                             <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                             <path d="M4.5 7L6 8.5L9.5 5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                           </svg>
                           <span>Activate an admin license to unlock stable and random-domain sharing.</span>
                         </div>
                       `}
                  <div class="action-row wide"><button type="submit">${e.editorMode==="create"?"Save Project":"Update Project"}</button></div>
                </form>
              </section>
            `:""}

          ${e.activeTab==="overview"?`
                <article class="panel selected-panel">
                  <div class="panel-header">
                    <div>
                      <h2>${i(a?.displayName||"No project selected")}</h2>
                    </div>
                    <div class="selected-header-actions">
                      ${a?`
                            <div class="dropdown">
                              <button type="button" class="secondary menu-button" data-action="toggle-project-menu">Manage</button>
                              ${e.projectMenuOpen?`
                                    <div class="dropdown-menu">
                                      <button type="button" class="dropdown-item" data-action="edit-project">
                                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                                          <path d="M11.333 2.667a.943.943 0 0 1 1.333 1.333l-6.4 6.4L3.333 11.333l.933-2.933 6.4-5.733Z" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
                                        </svg>
                                        <span>Edit project</span>
                                      </button>
                                      <button type="button" class="dropdown-item danger-item" data-action="delete-project">
                                        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
                                          <path d="M2.667 4h10.666M6 2.667h4M5.333 6v5.333M8 6v5.333M10.667 6v5.333M4.667 4l.4 8A1.333 1.333 0 0 0 6.4 13.333h3.2A1.333 1.333 0 0 0 10.933 12l.4-8" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>
                                        </svg>
                                        <span>Delete project</span>
                                      </button>
                                    </div>
                                  `:""}
                            </div>
                          `:""}
                    </div>
                  </div>

                  ${a?`
                        <div class="hero-project">
                          <div class="hero-project-main">
                            <strong>${i(a.localHost)}</strong>
                            <div class="inline-url-row">
                              <p>${i(o||p||"No public URL is available yet")}</p>
                              ${o||p?'<button type="button" class="secondary inline-copy-button" data-action="open-url" aria-label="Open public URL">Open</button>':""}
                              ${o||p?"":'<button type="button" class="secondary inline-copy-button" data-action="regenerate-url" aria-label="Generate public URL">Refresh URL</button>'}
                            </div>
                          </div>
                        </div>

                      `:'<p class="empty-copy">Create or select a project to start sharing.</p>'}
                </article>
              `:e.activeTab==="settings"&&b?`
                  <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Defaults</p>
                        <h2>App settings</h2>
                      </div>
                    </div>
                    <form id="settings-form" class="form-grid">
                      <label>Custom cloudflared path (optional)<input name="cloudflaredPath" value="${i(t.settings.cloudflaredPath)}" placeholder="Leave blank to use default" /></label>
                      <label>Local service URL<input name="defaultServiceURL" value="${i(t.settings.defaultServiceURL)}" /></label>
                      <div class="action-row wide"><button type="submit">Save Settings</button></div>
                    </form>
                  </article>

                  <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Tunnel context</p>
                        <h2>Routes and runtime</h2>
                      </div>
                      <span class="pill ${n.running?"pill-success":"pill-muted"}">${i(n.running?"running":"stopped")}</span>
                    </div>
                    <div class="status-grid two-column">
                      <div><label>Config file</label><strong>${i(t.configPath)}</strong></div>
                      <div><label>Cloudflared path</label><strong>${i(t.cloudflaredPath||"not detected")}</strong></div>
                    </div>
                    <div class="status-list">
                      <label>Loaded hostnames</label>
                      <div class="host-tags">
                        ${n.activeHostnames.length?n.activeHostnames.map(E=>`<span>${i(E)}</span>`).join(""):"<span>none</span>"}
                      </div>
                    </div>
                    <div class="action-row">
                      <button type="button" class="secondary" data-action="open-config">Open Config</button>
                      <button type="button" class="secondary" data-action="open-settings">Open Settings</button>
                    </div>
                    ${n.lastError?`<p class="error-copy">${i(n.lastError)}</p>`:""}
                  </article>
                 `:e.activeTab==="setup"?`
                    <article class="panel compact-panel">
                      <div class="panel-header">
                        <div>
                          <p class="eyebrow">License</p>
                          <h2>Offline activation</h2>
                        </div>
                        <span class="pill ${t.license.valid?"pill-success":"pill-outline"}">${i(t.license.valid?"active":"inactive")}</span>
                      </div>
                      <div class="status-grid two-column">
                        <div><label>Device ID</label><strong>${i(t.license.deviceId||"not detected")}</strong></div>
                        <div><label>Status</label><strong>${i(t.license.message||"No license activated")}</strong></div>
                        <div><label>Owner</label><strong>${i(t.license.owner||"—")}</strong></div>
                        <div><label>Plan</label><strong>${i(t.license.plan||"—")}</strong></div>
                        <div><label>Expires at</label><strong>${i(t.license.expiresAt||"never")}</strong></div>
                        <div><label>Admin features</label><strong>${i(t.license.isAdmin?"enabled":"disabled")}</strong></div>
                      </div>
                      <form id="license-form" class="form-grid">
                        <label>
                          Signed activation token
                          <textarea name="licenseToken" rows="5" placeholder="Paste signed offline activation token here">${i(e.licenseDraft)}</textarea>
                        </label>
                        <div class="action-row wide license-actions">
                          <button type="submit">Activate License</button>
                          ${t.license.configured?'<button type="button" class="secondary" data-action="clear-license">Remove License</button>':""}
                        </div>
                      </form>
                    </article>

                    <article class="panel compact-panel">
                      <div class="panel-header">
                        <div>
                          <p class="eyebrow">Setup</p>
                          <h2>Share tool and tunnel setup</h2>
                        </div>
                      </div>
                      <div class="metric-grid">
                        <div class="metric-card metric-card-split">
                          <div class="metric-card-copy">
                            <span class="summary-label">Share Tool</span>
                            <strong>${i(M)} <span class="pill ${A}">${i(M)}</span></strong>
                            <p>${i(N)}</p>
                          </div>
                          <div class="action-row">
                            <button type="button" class="${g?"secondary":""}" data-action="ensure-cloudflared">${g?"Reinstall Share Tool":"Install Share Tool"}</button>
                          </div>
                        </div>
                        <div class="metric-card metric-card-split">
                          <div class="metric-card-copy">
                            <span class="summary-label">Setup tunnel</span>
                            <strong>${i(T)} <span class="pill ${H}">${i(T)}</span></strong>
                            <p>${i(b?D:"Activate an admin license to unlock named tunnel setup and advanced settings.")}</p>
                          </div>
                          <div class="action-row">
                            <button type="button" class="secondary" data-action="create-tunnel" ${!m||!b?"disabled":""}>Setup Tunnel</button>
                          </div>
                        </div>
                      </div>
                    </article>
                  `:`
                  <section class="panel logs-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Logs</p>
                        <h2>cloudflared and npm output</h2>
                      </div>
                      <span class="pill pill-outline">${i(`${t.status.lastLogs.length} entries`)}</span>
                    </div>
                    <div class="log-stream">${X(t.status.lastLogs)}</div>
                  </section>
                `}

        </section>
      </section>

    </main>
  `,te()}function f(t,a){return t.elements.namedItem(a)?.value?.trim()??""}function k(){const t=w.querySelector("#project-form");if(!t)return;const a=f(t,"shareMode"),s=["quick","stable","random-domain"].includes(a)?a:"stable",r=f(t,"projectPath"),o=f(t,"localHost")||x(r);e.editorProject={id:f(t,"id"),displayName:f(t,"displayName"),localHost:o,projectPath:r,subdomain:f(t,"subdomain"),publicURL:e.editorProject.publicURL,shareMode:s}}async function l(t,a){try{L(t);const n=await a();return e.notice=null,n}catch(n){const s=n instanceof Error?n.message:String(n);d("error",s);return}finally{L(null)}}async function P(t){return!e.appState?.status.running||!e.activeProjectId||e.activeProjectId===t?!0:window.confirm("Another project is currently running. Stop the current project and continue with this one?")}function te(){const t=w.querySelector("#license-form");t?.addEventListener("submit",async s=>{s.preventDefault();const r=f(t,"licenseToken");e.licenseDraft=r;const o=await l("Activating license...",()=>c.activateLicense(r));o&&(e.appState=o,e.licenseDraft="",o.license.isAdmin&&e.activeTab!=="settings"&&(e.activeTab="settings"),d("success",o.license.message||"License activated"))});const a=w.querySelector("#settings-form");a?.addEventListener("submit",async s=>{s.preventDefault();const r={...e.appState.settings,cloudflaredPath:f(a,"cloudflaredPath"),defaultServiceURL:f(a,"defaultServiceURL")},o=await l("Saving settings...",()=>c.saveSettings(r));o&&(e.appState=o,d("success","Settings saved"))});const n=w.querySelector("#project-form");n?.addEventListener("change",s=>{const r=s.target;r&&(r.name==="shareMode"||r.name==="projectPath"||r.name==="localHost")&&(k(),u())}),n?.addEventListener("submit",async s=>{s.preventDefault();const r={id:f(n,"id"),displayName:f(n,"displayName"),localHost:f(n,"localHost"),projectPath:f(n,"projectPath"),subdomain:f(n,"subdomain"),publicURL:e.editorProject.publicURL,shareMode:f(n,"shareMode")||"stable"},o=await l("Saving project...",()=>c.saveProject(r));if(!o)return;e.appState=o;const p=r.id||(o.settings.projects[o.settings.projects.length-1]?.id??null);if(e.selectedProjectId=p,e.editorOpen=!1,!p){d("success","Project preset saved");return}if(!await P(p)){d("success","Project preset saved");return}if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==p){const m=await l("Stopping current project...",()=>c.stopTunnel());if(!m)return;e.appState=m,e.activeProjectId=null}const h=r.shareMode,g=await l(h==="quick"?"Creating public URL...":h==="random-domain"?"Generating random hostname...":"Sharing project...",()=>h==="quick"?c.startQuickTunnel(p):h==="random-domain"?c.shareProjectWithRandomURL(p):c.shareProject(p));if(g){e.appState=g,e.activeProjectId=p,d("success","Project saved and shared successfully");return}d("success","Project preset saved")})}async function ne(t,a){switch(t){case"tab-overview":e.activeTab="overview",e.projectMenuOpen=!1,u();return;case"tab-settings":if(!e.appState?.license.isAdmin){e.activeTab="setup",e.projectMenuOpen=!1,d("error","Activate an admin license to open Settings");return}e.activeTab="settings",e.projectMenuOpen=!1,u();return;case"tab-setup":e.activeTab="setup",e.projectMenuOpen=!1,u();return;case"tab-logs":e.activeTab="logs",e.projectMenuOpen=!1,u();return;case"toggle-project-menu":e.projectMenuOpen=!e.projectMenuOpen,u();return;case"select-project":e.selectedProjectId=a??null,e.projectMenuOpen=!1,u();return;case"new-project":e.editorOpen=!0,e.editorMode="create",e.editorProject=I(),u();return;case"edit-project":j()&&(e.editorOpen=!0,e.editorMode="edit",e.editorProject={...j()},e.projectMenuOpen=!1,u());return;case"close-editor":e.editorOpen=!1,u();return;case"clear-license":{const n=await l("Removing license...",()=>c.clearLicense());n&&(e.appState=n,e.licenseDraft="",e.activeTab==="settings"&&(e.activeTab="setup"),d("success","License removed"));return}case"browse-project-folder":{k();const n=await l("Opening folder picker...",()=>c.browseProjectFolder(e.editorProject.projectPath));if(typeof n=="string"&&n){const s=x(n);e.editorProject.projectPath=n,e.editorProject.localHost.trim()||(e.editorProject.localHost=s),u()}return}case"random-subdomain":{k(),e.editorProject.subdomain=Y(),u();return}case"copy-url":{e.projectMenuOpen=!1;const n=j();if(!n||!e.appState)return;const s=O(n,e.appState.settings.defaultDomain);if(!s){d("error","No public URL is available for the selected project");return}await navigator.clipboard.writeText(s),e.copiedProjectId=n.id,y!==null&&window.clearTimeout(y),y=window.setTimeout(()=>{e.copiedProjectId=null,y=null,u()},1800),u();return}}if(!a&&["share-project","share-random","share-quick","regenerate-url","open-url","npm-build","test-project","delete-project"].includes(t)){d("error","Select a project first");return}switch(t){case"start-tunnel":{const n=await l("Checking tunnel state...",()=>c.refreshState());if(!n)return;if(e.appState=n,n.status.running){d("info","A tunnel is already running. Stop it first before starting again.");return}try{L("Starting tunnel...");const s=await c.startTunnel();e.appState=s,d("success","Named tunnel started")}catch(s){const r=s instanceof Error?s.message:String(s);if(r.toLowerCase().includes("already running")){const o=await c.refreshState().catch(()=>null);o&&(e.appState=o),d("info","A tunnel is already running. Stop it first before starting again.")}else d("error",r)}finally{L(null)}return}case"ensure-cloudflared":{const n=await l("Installing share tool...",()=>c.ensureCloudflared());n&&(e.appState=n,d("success","Share tool installed. You can now set up the tunnel."));return}case"stop-tunnel":{const n=await l("Stopping tunnel...",()=>c.stopTunnel());if(n){const s=e.appState?.settings.projects.find(r=>r.id===e.activeProjectId);if(e.appState=n,s?.shareMode==="quick"){delete e.projectUrls[s.id];const r={...s,publicURL:""};c.saveProject(r).then(o=>{e.appState=o,u()}).catch(()=>{})}e.activeProjectId=null,d("success","Tunnel stopped")}return}case"create-tunnel":{const n=await l("Ensuring named tunnel...",()=>c.createTunnel());n&&(e.appState=n,d("success","Named tunnel is ready"));return}case"open-config":await l("Opening config file...",()=>c.openConfigFile());return;case"open-settings":await l("Opening settings file...",()=>c.openSettingsFile());return;case"refresh":{const n=await l("Refreshing state...",()=>c.refreshState());n&&(e.appState=n,!e.activeProjectId&&n.status.running&&(e.activeProjectId=U(n)),d("success","State refreshed"));return}case"share-project":{if(!await P(a))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==a){const s=await l("Stopping current project...",()=>c.stopTunnel());if(!s)return;e.appState=s,e.activeProjectId=null}const n=await l("Sharing project...",()=>c.shareProject(a));n&&(e.appState=n,e.activeProjectId=a,(n.status.activeUrl||n.status.quickUrl)&&(e.projectUrls[a]=n.status.activeUrl||n.status.quickUrl),d("success","Project shared through named tunnel"));return}case"share-random":{if(!await P(a))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==a){const s=await l("Stopping current project...",()=>c.stopTunnel());if(!s)return;e.appState=s,e.activeProjectId=null}const n=await l("Generating random hostname...",()=>c.shareProjectWithRandomURL(a));if(n){if(e.appState=n,e.activeProjectId=a,n.settings.projects){const s=n.settings.projects.find(r=>r.id===a);s?.publicURL&&(e.projectUrls[a]=s.publicURL)}d("success","Random domain share is active")}return}case"share-quick":{if(!await P(a))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==a){const s=await l("Stopping current project...",()=>c.stopTunnel());if(!s)return;e.appState=s,e.activeProjectId=null}const n=await l("Creating public URL...",()=>c.startQuickTunnel(a));n&&(e.appState=n,e.activeProjectId=a,(n.status.activeUrl||n.status.quickUrl)&&(e.projectUrls[a]=n.status.activeUrl||n.status.quickUrl),d("success","Public URL is live"));return}case"regenerate-url":{const n=j();if(!n||!await P(n.id))return;if(e.appState?.status.running){const r=await l("Stopping current project...",()=>c.stopTunnel());if(!r)return;e.appState=r,e.activeProjectId=null}const s=await l(n.shareMode==="quick"?"Generating new public URL...":"Generating new random URL...",()=>n.shareMode==="quick"?c.startQuickTunnel(n.id):c.shareProjectWithRandomURL(n.id));if(s){if(e.appState=s,e.activeProjectId=n.id,(s.status.activeUrl||s.status.quickUrl)&&(e.projectUrls[n.id]=s.status.activeUrl||s.status.quickUrl),s.settings.projects){const r=s.settings.projects.find(o=>o.id===n.id);r?.publicURL&&(e.projectUrls[n.id]=r.publicURL)}d("success","New public URL generated")}return}case"open-url":await l("Opening public URL...",()=>c.openPublicURL(a));return;case"npm-build":{await l("Running npm build...",()=>c.runNpmBuild(a))!==void 0&&d("success","npm build started. Watch the log stream for progress");return}case"test-project":{const n=await l("Testing local URL...",()=>c.testProject(a));typeof n=="string"&&d("success",n);return}case"delete-project":if(window.confirm("Delete this project preset?")){const n=await l("Deleting project...",()=>c.deleteProject(a));n&&(e.appState=n,e.selectedProjectId=n.settings.projects[0]?.id??null,e.projectMenuOpen=!1,d("success","Project deleted"))}return}}w.addEventListener("click",t=>{const a=t.target,n=a?.closest(".dropdown");if(e.projectMenuOpen&&!n){e.projectMenuOpen=!1,u();return}const s=a?.closest("[data-action]");if(!s)return;t.preventDefault();const r=s.dataset.action,o=s.dataset.id??j()?.id??null;r&&ne(r,o)});async function ae(){try{await G()&&V(),_(Z,K),F(z,Q),B()}catch{}const t=await l("Loading app state...",()=>c.bootstrap());t&&(e.appState=t,$(t),e.selectedProjectId=t.settings.projects[0]?.id??null,e.activeProjectId=U(t),u(),window.runtime?.EventsOn("log",a=>{if(!e.appState)return;const n=a;e.appState.status.lastLogs=[...e.appState.status.lastLogs,n].slice(-300),u()}),window.runtime?.EventsOn("status",a=>{if(!e.appState)return;const n=a;if(e.appState.status=n,!e.activeProjectId&&n.running&&(e.activeProjectId=U(e.appState)),e.activeProjectId&&(n.activeUrl||n.quickUrl)){const s=n.activeUrl||n.quickUrl,r=e.projectUrls[e.activeProjectId];if(e.projectUrls[e.activeProjectId]=s,s&&s!==r){const o=e.appState.settings.projects.find(p=>p.id===e.activeProjectId);if(o&&o.shareMode!=="quick"&&o.publicURL!==s){const p={...o,publicURL:s};c.saveProject(p).then(h=>{e.appState=h,$(h),u()}).catch(()=>{})}}}u()}))}ae();
