(function(){const a=document.createElement("link").relList;if(a&&a.supports&&a.supports("modulepreload"))return;for(const o of document.querySelectorAll('link[rel="modulepreload"]'))r(o);new MutationObserver(o=>{for(const i of o)if(i.type==="childList")for(const f of i.addedNodes)f.tagName==="LINK"&&f.rel==="modulepreload"&&r(f)}).observe(document,{childList:!0,subtree:!0});function n(o){const i={};return o.integrity&&(i.integrity=o.integrity),o.referrerPolicy&&(i.referrerPolicy=o.referrerPolicy),o.crossOrigin==="use-credentials"?i.credentials="include":o.crossOrigin==="anonymous"?i.credentials="omit":i.credentials="same-origin",i}function r(o){if(o.ep)return;o.ep=!0;const i=n(o);fetch(o.href,i)}})();function p(){const t=window.go?.main?.App;if(!t)throw new Error("Wails bindings are not available. Run the project with `wails dev` or `wails build`.");return t}const c={bootstrap:()=>p().Bootstrap(),refreshState:()=>p().RefreshState(),ensureCloudflared:()=>p().EnsureCloudflared(),saveSettings:t=>p().SaveSettings(t),saveProject:t=>p().SaveProject(t),deleteProject:t=>p().DeleteProject(t),shareProject:t=>p().ShareProject(t),shareProjectWithRandomURL:t=>p().ShareProjectWithRandomURL(t),startQuickTunnel:t=>p().StartQuickTunnel(t),startTunnel:()=>p().StartTunnel(),stopTunnel:()=>p().StopTunnel(),createTunnel:()=>p().CreateTunnel(),runNpmBuild:t=>p().RunNpmBuild(t),openPublicURL:t=>p().OpenPublicURL(t),openConfigFile:()=>p().OpenConfigFile(),openSettingsFile:()=>p().OpenSettingsFile(),browseProjectFolder:t=>p().BrowseProjectFolder(t),testProject:t=>p().TestProject(t)};function O(){window.runtime.WindowCenter()}function C(t,a){window.runtime.WindowSetSize(t,a)}function x(t,a){window.runtime.WindowSetMinSize(t,a)}function N(){window.runtime.WindowUnmaximise()}function H(){return window.runtime.WindowIsMaximised()}const M=()=>({id:"",displayName:"",localHost:"",subdomain:"",publicURL:"",projectPath:"",shareMode:"quick"}),e={appState:null,selectedProjectId:null,activeProjectId:null,projectUrls:{},activeTab:"overview",projectMenuOpen:!1,editorOpen:!1,editorMode:"create",editorProject:M(),notice:null,busy:null},U=document.querySelector("#app");if(!U)throw new Error("App root not found");const m=U,W=1040,q=700,D=900,E=620;let w=null;function s(t){return t.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;")}function $(t,a){return t.publicURL?t.publicURL:t.subdomain&&t.shareMode!=="quick"?`https://${t.subdomain}.${a}`:""}function T(t){const a=t.trim().replace(/[/\\]+$/,"");if(!a)return"";const n=a.split(/[/\\]+/),r=n[n.length-1]?.trim()??"";if(!r)return"";const o=r.toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-+|-+$/g,"");return o?`${o}.test`:""}function A(){return Math.random().toString(36).slice(2,8)}function h(){const t=e.appState?.settings.projects??[];return t.length===0?null:e.selectedProjectId?t.find(a=>a.id===e.selectedProjectId)??t[0]:t[0]}function k(t){for(const a of t.settings.projects)a.publicURL?.trim()&&(e.projectUrls[a.id]=a.publicURL.trim())}function d(t,a){e.notice={tone:t,message:a},w!==null&&window.clearTimeout(w),w=window.setTimeout(()=>{e.notice=null,w=null,u()},3e3),u()}function P(t){e.busy=t,u()}function F(t){return t.running?"success":t.lastError?"error":"idle"}function B(t){return t.slice(-12).reverse().map(a=>`
        <div class="log-entry log-${s(a.level)}">
          <span>${s(new Date(a.timestamp).toLocaleTimeString())}</span>
          <strong>${s(a.source)}</strong>
          <p>${s(a.message)}</p>
        </div>
      `).join("")}function _(t){return t.settings.projects.map(a=>{const n=h()?.id===a.id,r=(a.displayName||a.localHost||"P").slice(0,2).toUpperCase(),o=e.activeProjectId===a.id&&t.status.running;return`
        <button type="button" class="project-row ${n?"selected":""}" data-action="select-project" data-id="${s(a.id)}">
          <div class="project-avatar">${s(r)}</div>
          <div class="project-copy">
            <strong>${s(a.displayName)}</strong>
            <span>${s(a.localHost)}</span>
          </div>
          ${o?'<span class="project-running-badge">Running</span>':""}
        </button>
      `}).join("")}function u(){if(!e.appState){m.innerHTML=`
      <main class="shell loading-state">
        <section class="hero-card">
          <h1>Cloudflare Tunnel Manager</h1>
          <p>Loading backend state...</p>
        </section>
      </main>
    `;return}const t=e.appState;k(t);const a=h();e.selectedProjectId=a?.id??null;const n=t.status,r=a&&(e.projectUrls[a.id]||a.publicURL)||"",i=e.activeProjectId===a?.id&&(n.activeUrl||n.quickUrl)||""||r,f=a?$(a,t.settings.defaultDomain):"",b=t.settings.projects.length>0,g=t.cloudflaredDetected,y=g,R=g&&b,L=g?b?"":"Create a project first":"Prepare Share Tool first";m.innerHTML=`
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
              <p>Manager</p>
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
          <button type="button" class="nav-item ${e.activeTab==="settings"?"active":""}" data-action="tab-settings">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="10" cy="10" r="3" stroke="currentColor" stroke-width="2"/>
              <path d="M10 2V4M10 16V18M18 10H16M4 10H2M15.66 4.34L14.24 5.76M5.76 14.24L4.34 15.66M15.66 15.66L14.24 14.24M5.76 5.76L4.34 4.34" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
            <span>Settings</span>
          </button>
          <button type="button" class="nav-item ${e.activeTab==="setup"?"active":""}" data-action="tab-setup">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M10 3L16 6.5V13.5L10 17L4 13.5V6.5L10 3Z" stroke="currentColor" stroke-width="2" stroke-linejoin="round"/>
              <path d="M10 8V10.5" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
              <circle cx="10" cy="13" r="1" fill="currentColor"/>
            </svg>
            <span>Setup</span>
          </button>
          <button type="button" class="nav-item ${e.activeTab==="logs"?"active":""}" data-action="tab-logs">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <path d="M3 5H17M3 10H17M3 15H17" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
            <span>Logs</span>
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
          <div class="project-list">${_(t)}</div>
        </div>

        <div class="sidebar-footer">
          <div class="status-indicator tone-${F(n)}">
            <span class="status-dot"></span>
            <span>${s(n.running?"Running":"Stopped")}</span>
          </div>
        </div>
      </aside>

      <section class="main-content">
        <header class="content-header">
          <div class="header-info">
            <h2>${e.activeTab==="overview"?"Overview":e.activeTab==="settings"?"Settings":e.activeTab==="setup"?"Setup":"Logs"}</h2>
          </div>
          <div class="header-actions">
            <button type="button" data-action="start-tunnel" ${e.appState?.status.running||!R?"disabled":""}>Start Tunnel</button>
            <button type="button" class="secondary" data-action="stop-tunnel">Stop</button>
            <button type="button" class="secondary" data-action="refresh">Refresh</button>
          </div>
        </header>

        ${L?`<p class="header-hint">${s(L)}</p>`:""}

        ${e.notice?`<section class="toast toast-${e.notice.tone}">${s(e.notice.message)}</section>`:""}

          ${e.editorOpen?`
              <section class="panel editor-panel">
                <div class="panel-header">
                  <div>
                    <p class="eyebrow">${e.editorMode==="create"?"New project":"Edit project"}</p>
                    <h2>${s(e.editorProject.displayName||"Project preset")}</h2>
                  </div>
                  <button type="button" class="secondary" data-action="close-editor">Close</button>
                </div>
                <form id="project-form" class="form-grid editor-grid">
                  <input type="hidden" name="id" value="${s(e.editorProject.id)}" />
                  <label>Display name<input name="displayName" value="${s(e.editorProject.displayName)}" /></label>
                  <label>
                    Project folder
                    <div class="folder-picker">
                      <input name="projectPath" value="${s(e.editorProject.projectPath)}" placeholder="D:\\code\\hr-system" />
                      <button type="button" class="secondary browse-button" data-action="browse-project-folder">Browse</button>
                    </div>
                  </label>
                  <label>Local host<input name="localHost" value="${s(e.editorProject.localHost)}" placeholder="hr-system.test" /></label>
                  <label>
                    Share mode
                    <select name="shareMode">
                      <option value="quick" ${e.editorProject.shareMode==="quick"?"selected":""}>One-click public URL</option>
                      <option value="stable" ${e.editorProject.shareMode==="stable"?"selected":""}>Stable hostname</option>
                      <option value="random-domain" ${e.editorProject.shareMode==="random-domain"?"selected":""}>Random under my domain</option>
                    </select>
                  </label>
                  ${e.editorProject.shareMode==="stable"?`
                        <label>
                          Stable subdomain
                          <div class="folder-picker">
                            <input name="subdomain" value="${s(e.editorProject.subdomain)}" placeholder="app" />
                            <button type="button" class="secondary browse-button" data-action="random-subdomain">Random</button>
                          </div>
                        </label>
                      `:""}
                  <div class="action-row wide"><button type="submit">${e.editorMode==="create"?"Save Project":"Update Project"}</button></div>
                </form>
              </section>
            `:""}

          ${e.activeTab==="overview"?`
                <article class="panel selected-panel">
                  <div class="panel-header">
                    <div>
                      <h2>${s(a?.displayName||"No project selected")}</h2>
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
                            <strong>${s(a.localHost)}</strong>
                            <div class="inline-url-row">
                              <p>${s(i||f||"No public URL is available yet")}</p>
                              ${i||f?'<button type="button" class="secondary inline-copy-button" data-action="copy-url" aria-label="Copy public URL">Copy</button>':""}
                            </div>
                          </div>
                        </div>

                      `:'<p class="empty-copy">Create or select a project to start sharing.</p>'}
                </article>
              `:e.activeTab==="settings"?`
                  <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Defaults</p>
                        <h2>App settings</h2>
                      </div>
                    </div>
                    <form id="settings-form" class="form-grid">
                      <label>Default domain<input name="defaultDomain" value="${s(t.settings.defaultDomain)}" /></label>
                      <label>Tunnel name<input name="tunnelName" value="${s(t.settings.tunnelName)}" /></label>
                      <label>Cloudflared path<input name="cloudflaredPath" value="${s(t.settings.cloudflaredPath)}" placeholder="Leave blank to use PATH" /></label>
                      <label>Local service URL<input name="defaultServiceURL" value="${s(t.settings.defaultServiceURL)}" /></label>
                      <label>Managed cloudflared location<input value="${s(t.managedCloudflaredPath)}" disabled /></label>
                      <div class="action-row wide"><button type="submit">Save Settings</button></div>
                    </form>
                  </article>

                  <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Tunnel context</p>
                        <h2>Routes and runtime</h2>
                      </div>
                      <span class="pill ${n.running?"pill-success":"pill-muted"}">${s(n.running?"running":"stopped")}</span>
                    </div>
                    <div class="status-grid two-column">
                      <div><label>Config file</label><strong>${s(t.configPath)}</strong></div>
                      <div><label>Cloudflared path</label><strong>${s(t.cloudflaredPath||"not detected")}</strong></div>
                    </div>
                    <div class="status-list">
                      <label>Loaded hostnames</label>
                      <div class="host-tags">
                        ${n.activeHostnames.length?n.activeHostnames.map(I=>`<span>${s(I)}</span>`).join(""):"<span>none</span>"}
                      </div>
                    </div>
                    <div class="action-row">
                      <button type="button" class="secondary" data-action="open-config">Open Config</button>
                      <button type="button" class="secondary" data-action="open-settings">Open Settings</button>
                    </div>
                    ${n.lastError?`<p class="error-copy">${s(n.lastError)}</p>`:""}
                  </article>
                `:e.activeTab==="setup"?`
                    <article class="panel compact-panel">
                      <div class="panel-header">
                        <div>
                          <p class="eyebrow">Setup</p>
                          <h2>Share tool and tunnel setup</h2>
                        </div>
                      </div>
                      <div class="metric-grid">
                        <div class="metric-card">
                          <span class="summary-label">Prepare share tool</span>
                          <strong>${s(t.cloudflaredDetected?"Ready":"Required")}</strong>
                          <p>Prepare cloudflared before starting or sharing projects.</p>
                          <div class="action-row">
                            <button type="button" class="${g?"secondary":""}" data-action="ensure-cloudflared">Prepare Share Tool</button>
                          </div>
                        </div>
                        <div class="metric-card">
                          <span class="summary-label">Setup tunnel</span>
                          <strong>${s(y?"Available":"Waiting")}</strong>
                          <p>Create or reuse the named tunnel after the share tool is ready.</p>
                          <div class="action-row">
                            <button type="button" class="secondary" data-action="create-tunnel" ${y?"":"disabled"}>Setup Tunnel</button>
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
                      <span class="pill pill-outline">${s(`${t.status.lastLogs.length} entries`)}</span>
                    </div>
                    <div class="log-stream">${B(t.status.lastLogs)}</div>
                  </section>
                `}

        </section>
      </section>

    </main>
  `,V()}function v(t,a){return t.elements.namedItem(a)?.value?.trim()??""}function S(){const t=m.querySelector("#project-form");if(!t)return;const a=v(t,"shareMode"),r=["quick","stable","random-domain"].includes(a)?a:"stable",o=v(t,"projectPath"),i=v(t,"localHost")||T(o);e.editorProject={id:v(t,"id"),displayName:v(t,"displayName"),localHost:i,projectPath:o,subdomain:v(t,"subdomain"),publicURL:e.editorProject.publicURL,shareMode:r}}async function l(t,a){try{P(t);const n=await a();return e.notice=null,n}catch(n){const r=n instanceof Error?n.message:String(n);d("error",r);return}finally{P(null)}}async function j(t){return!e.appState?.status.running||!e.activeProjectId||e.activeProjectId===t?!0:window.confirm("Another project is currently running. Stop the current project and continue with this one?")}function V(){const t=m.querySelector("#settings-form");t?.addEventListener("submit",async n=>{n.preventDefault();const r={...e.appState.settings,defaultDomain:v(t,"defaultDomain"),tunnelName:v(t,"tunnelName"),cloudflaredPath:v(t,"cloudflaredPath"),defaultServiceURL:v(t,"defaultServiceURL")},o=await l("Saving settings...",()=>c.saveSettings(r));o&&(e.appState=o,d("success","Settings saved"))});const a=m.querySelector("#project-form");a?.addEventListener("change",n=>{const r=n.target;r&&(r.name==="shareMode"||r.name==="projectPath"||r.name==="localHost")&&(S(),u())}),a?.addEventListener("submit",async n=>{n.preventDefault();const r={id:v(a,"id"),displayName:v(a,"displayName"),localHost:v(a,"localHost"),projectPath:v(a,"projectPath"),subdomain:v(a,"subdomain"),publicURL:e.editorProject.publicURL,shareMode:v(a,"shareMode")||"stable"},o=await l("Saving project...",()=>c.saveProject(r));if(!o)return;e.appState=o;const i=r.id||(o.settings.projects[o.settings.projects.length-1]?.id??null);if(e.selectedProjectId=i,e.editorOpen=!1,!i){d("success","Project preset saved");return}if(!await j(i)){d("success","Project preset saved");return}if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==i){const g=await l("Stopping current project...",()=>c.stopTunnel());if(!g)return;e.appState=g,e.activeProjectId=null}const f=r.shareMode,b=await l(f==="quick"?"Creating public URL...":f==="random-domain"?"Generating random hostname...":"Sharing project...",()=>f==="quick"?c.startQuickTunnel(i):f==="random-domain"?c.shareProjectWithRandomURL(i):c.shareProject(i));if(b){e.appState=b,e.activeProjectId=i,d("success","Project saved and shared successfully");return}d("success","Project preset saved")})}async function z(t,a){switch(t){case"tab-overview":e.activeTab="overview",e.projectMenuOpen=!1,u();return;case"tab-settings":e.activeTab="settings",e.projectMenuOpen=!1,u();return;case"tab-setup":e.activeTab="setup",e.projectMenuOpen=!1,u();return;case"tab-logs":e.activeTab="logs",e.projectMenuOpen=!1,u();return;case"toggle-project-menu":e.projectMenuOpen=!e.projectMenuOpen,u();return;case"select-project":e.selectedProjectId=a??null,e.projectMenuOpen=!1,u();return;case"new-project":e.editorOpen=!0,e.editorMode="create",e.editorProject=M(),u();return;case"edit-project":h()&&(e.editorOpen=!0,e.editorMode="edit",e.editorProject={...h()},e.projectMenuOpen=!1,u());return;case"close-editor":e.editorOpen=!1,u();return;case"browse-project-folder":{S();const n=await l("Opening folder picker...",()=>c.browseProjectFolder(e.editorProject.projectPath));if(typeof n=="string"&&n){const r=T(n);e.editorProject.projectPath=n,e.editorProject.localHost.trim()||(e.editorProject.localHost=r),u()}return}case"random-subdomain":{S(),e.editorProject.subdomain=A(),u();return}case"copy-url":{e.projectMenuOpen=!1;const n=h();if(!n||!e.appState)return;const r=$(n,e.appState.settings.defaultDomain);if(!r){d("error","No public URL is available for the selected project");return}await navigator.clipboard.writeText(r);return}}if(!a&&["share-project","share-random","share-quick","regenerate-url","open-url","npm-build","test-project","delete-project"].includes(t)){d("error","Select a project first");return}switch(t){case"start-tunnel":{const n=await l("Checking tunnel state...",()=>c.refreshState());if(!n)return;if(e.appState=n,n.status.running){d("info","A tunnel is already running. Stop it first before starting again.");return}try{P("Starting tunnel...");const r=await c.startTunnel();e.appState=r,d("success","Named tunnel started")}catch(r){const o=r instanceof Error?r.message:String(r);if(o.toLowerCase().includes("already running")){const i=await c.refreshState().catch(()=>null);i&&(e.appState=i),d("info","A tunnel is already running. Stop it first before starting again.")}else d("error",o)}finally{P(null)}return}case"ensure-cloudflared":{const n=await l("Preparing cloudflared...",()=>c.ensureCloudflared());n&&(e.appState=n,d("success","cloudflared is ready for one-click sharing"));return}case"stop-tunnel":{const n=await l("Stopping tunnel...",()=>c.stopTunnel());n&&(e.appState=n,e.activeProjectId=null,d("success","Tunnel stopped"));return}case"create-tunnel":{const n=await l("Ensuring named tunnel...",()=>c.createTunnel());n&&(e.appState=n,d("success","Named tunnel is ready"));return}case"open-config":await l("Opening config file...",()=>c.openConfigFile());return;case"open-settings":await l("Opening settings file...",()=>c.openSettingsFile());return;case"refresh":{const n=await l("Refreshing state...",()=>c.refreshState());n&&(e.appState=n,d("success","State refreshed"));return}case"share-project":{if(!await j(a))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==a){const r=await l("Stopping current project...",()=>c.stopTunnel());if(!r)return;e.appState=r,e.activeProjectId=null}const n=await l("Sharing project...",()=>c.shareProject(a));n&&(e.appState=n,e.activeProjectId=a,(n.status.activeUrl||n.status.quickUrl)&&(e.projectUrls[a]=n.status.activeUrl||n.status.quickUrl),d("success","Project shared through named tunnel"));return}case"share-random":{if(!await j(a))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==a){const r=await l("Stopping current project...",()=>c.stopTunnel());if(!r)return;e.appState=r,e.activeProjectId=null}const n=await l("Generating random hostname...",()=>c.shareProjectWithRandomURL(a));if(n){if(e.appState=n,e.activeProjectId=a,n.settings.projects){const r=n.settings.projects.find(o=>o.id===a);r?.publicURL&&(e.projectUrls[a]=r.publicURL)}d("success","Random domain share is active")}return}case"share-quick":{if(!await j(a))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==a){const r=await l("Stopping current project...",()=>c.stopTunnel());if(!r)return;e.appState=r,e.activeProjectId=null}const n=await l("Creating public URL...",()=>c.startQuickTunnel(a));n&&(e.appState=n,e.activeProjectId=a,(n.status.activeUrl||n.status.quickUrl)&&(e.projectUrls[a]=n.status.activeUrl||n.status.quickUrl),d("success","Public URL is live"));return}case"regenerate-url":{const n=h();if(!n||!await j(n.id))return;if(e.appState?.status.running){const o=await l("Stopping current project...",()=>c.stopTunnel());if(!o)return;e.appState=o,e.activeProjectId=null}const r=await l(n.shareMode==="quick"?"Generating new public URL...":"Generating new random URL...",()=>n.shareMode==="quick"?c.startQuickTunnel(n.id):c.shareProjectWithRandomURL(n.id));if(r){if(e.appState=r,e.activeProjectId=n.id,(r.status.activeUrl||r.status.quickUrl)&&(e.projectUrls[n.id]=r.status.activeUrl||r.status.quickUrl),r.settings.projects){const o=r.settings.projects.find(i=>i.id===n.id);o?.publicURL&&(e.projectUrls[n.id]=o.publicURL)}d("success","New public URL generated")}return}case"open-url":await l("Opening public URL...",()=>c.openPublicURL(a));return;case"npm-build":{await l("Running npm build...",()=>c.runNpmBuild(a))!==void 0&&d("success","npm build started. Watch the log stream for progress");return}case"test-project":{const n=await l("Testing local URL...",()=>c.testProject(a));typeof n=="string"&&d("success",n);return}case"delete-project":if(window.confirm("Delete this project preset?")){const n=await l("Deleting project...",()=>c.deleteProject(a));n&&(e.appState=n,e.selectedProjectId=n.settings.projects[0]?.id??null,e.projectMenuOpen=!1,d("success","Project deleted"))}return}}m.addEventListener("click",t=>{const a=t.target,n=a?.closest(".dropdown");if(e.projectMenuOpen&&!n){e.projectMenuOpen=!1,u();return}const r=a?.closest("[data-action]");if(!r)return;t.preventDefault();const o=r.dataset.action,i=r.dataset.id??h()?.id??null;o&&z(o,i)});async function G(){try{await H()&&N(),x(D,E),C(W,q),O()}catch{}const t=await l("Loading app state...",()=>c.bootstrap());t&&(e.appState=t,k(t),e.selectedProjectId=t.settings.projects[0]?.id??null,e.activeProjectId=null,u(),window.runtime?.EventsOn("log",a=>{if(!e.appState)return;const n=a;e.appState.status.lastLogs=[...e.appState.status.lastLogs,n].slice(-300),u()}),window.runtime?.EventsOn("status",a=>{if(!e.appState)return;const n=a;e.appState.status=n,e.activeProjectId&&(n.activeUrl||n.quickUrl)&&(e.projectUrls[e.activeProjectId]=n.activeUrl||n.quickUrl),u()}))}G();
