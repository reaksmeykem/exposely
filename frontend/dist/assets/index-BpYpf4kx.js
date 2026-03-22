(function(){const n=document.createElement("link").relList;if(n&&n.supports&&n.supports("modulepreload"))return;for(const a of document.querySelectorAll('link[rel="modulepreload"]'))o(a);new MutationObserver(a=>{for(const s of a)if(s.type==="childList")for(const p of s.addedNodes)p.tagName==="LINK"&&p.rel==="modulepreload"&&o(p)}).observe(document,{childList:!0,subtree:!0});function r(a){const s={};return a.integrity&&(s.integrity=a.integrity),a.referrerPolicy&&(s.referrerPolicy=a.referrerPolicy),a.crossOrigin==="use-credentials"?s.credentials="include":a.crossOrigin==="anonymous"?s.credentials="omit":s.credentials="same-origin",s}function o(a){if(a.ep)return;a.ep=!0;const s=r(a);fetch(a.href,s)}})();function h(){const t=window.go?.main?.App;if(!t)throw!!window.wails||!!window.runtime?new Error("Wails bindings are not yet available. This might happen if Bootstrap is called too early."):new Error("Wails bindings are not available. Please ensure you are running the application via Wails.");return t}const c={bootstrap:()=>h().Bootstrap(),refreshState:()=>h().RefreshState(),ensureCloudflared:()=>h().EnsureCloudflared(),installCloudflared:()=>h().InstallCloudflared(),saveSettings:t=>h().SaveSettings(t),saveProject:t=>h().SaveProject(t),deleteProject:t=>h().DeleteProject(t),shareProject:t=>h().ShareProject(t),shareProjectWithRandomURL:t=>h().ShareProjectWithRandomURL(t),startQuickTunnel:t=>h().StartQuickTunnel(t),startTunnel:()=>h().StartTunnel(),stopTunnel:()=>h().StopTunnel(),createTunnel:()=>h().CreateTunnel(),activateLicense:t=>h().ActivateLicense(t),clearLicense:()=>h().ClearLicense(),runNpmBuild:t=>h().RunNpmBuild(t),openPublicURL:t=>h().OpenPublicURL(t),openConfigFile:()=>h().OpenConfigFile(),openSettingsFile:()=>h().OpenSettingsFile(),browseProjectFolder:t=>h().BrowseProjectFolder(t),testProject:t=>h().TestProject(t)};function D(){window.runtime.WindowCenter()}function F(t,n){window.runtime.WindowSetSize(t,n)}function V(t,n){window.runtime.WindowSetMinSize(t,n)}function _(){window.runtime.WindowUnmaximise()}function z(){return window.runtime.WindowIsMaximised()}const $=()=>({id:"",displayName:"",localHost:"",subdomain:"",publicURL:"",projectPath:"",localURL:"",startCommand:"",shareMode:"auto"}),e={appState:null,fatalError:null,selectedProjectId:null,activeProjectId:null,projectUrls:{},copiedProjectId:null,activeTab:"overview",projectMenuOpen:!1,editorOpen:!1,editorMode:"create",editorProject:$(),notice:null,busy:null,licenseDraft:""},C=document.querySelector("#app");if(!C)throw new Error("App root not found");const m=C,G=1040,Q=700,K=900,Z=620;let w=null,P=null;function i(t){return t.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;")}function J(t,n){return t.publicURL?t.publicURL:t.subdomain?`https://${t.subdomain}.${n}`:""}function I(t,n){const r=e.activeProjectId===t.id?(n.status.activeUrl||n.status.quickUrl||"").trim():"";if(r)return r;const o=e.projectUrls[t.id]?.trim()||"";return o||J(t,n.settings.defaultDomain).trim()}function x(t){switch(t.shareMode){case"auto":return{action:"share-quick",label:"Start Auto Share"};case"quick":return{action:"share-quick",label:"Create Public URL"};case"host-html":return{action:"share-quick",label:"Create HTML Site URL"};default:return{action:"share-quick",label:"Create Public URL"}}}function L(t){return/^https?:\/\//i.test(t.trim())}function O(t){const n=t.trim().replace(/[/\\]+$/,"");if(!n||L(n))return"";const r=n.split(/[/\\]+/),o=r[r.length-1]?.trim()??"";if(!o)return"";const a=o.toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-+|-+$/g,"");return a?`${a}.test`:""}function X(t){switch(t.shareMode){case"auto":return"Auto Project";case"host-html":return"HTML Project";default:return"Laravel Project"}}function Y(t){return t.localURL.trim()?t.localURL.trim():L(t.projectPath)?t.projectPath.trim():t.shareMode==="host-html"?"HTML Site (Folder)":t.localHost.trim()?t.localHost.trim():t.startCommand.trim()?`Auto via ${t.startCommand.trim()}`:t.projectPath.trim()||"Project source"}function ee(){return Math.random().toString(36).slice(2,8)}function g(){const t=e.appState?.settings.projects??[];return t.length===0?null:e.selectedProjectId?t.find(n=>n.id===e.selectedProjectId)??t[0]:t[0]}function S(t){for(const n of t.settings.projects)n.publicURL?.trim()&&(e.projectUrls[n.id]=n.publicURL.trim())}function k(t){const n=(t.status.activeUrl||t.status.quickUrl||"").trim();if(!t.status.running)return null;if(n){const r=t.settings.projects.find(o=>(e.projectUrls[o.id]||o.publicURL||"").trim()===n);if(r)return r.id}return e.selectedProjectId||t.settings.projects[0]?.id||null}function d(t,n){e.notice={tone:t,message:n},w!==null&&window.clearTimeout(w),w=window.setTimeout(()=>{e.notice=null,w=null,u()},3e3),u()}function y(t){e.busy=t,u()}function te(t){return t.running?"success":t.lastError?"error":"idle"}function re(t){return t.slice(-12).reverse().map(n=>`
        <div class="log-entry log-${i(n.level)}">
          <span>${i(new Date(n.timestamp).toLocaleTimeString())}</span>
          <strong>${i(n.source)}</strong>
          <p>${i(n.message)}</p>
        </div>
      `).join("")}function ne(t){return t.settings.projects.map(n=>{const r=g()?.id===n.id,o=e.activeProjectId===n.id&&t.status.running;return`
        <button type="button" class="project-row ${r?"selected":""}" data-action="select-project" data-id="${i(n.id)}">
          <div class="project-copy">
            <strong>${i(n.displayName)}</strong>
            <span class="project-type-tag">${i(X(n))}</span>
          </div>
          ${o?'<span class="project-running-badge">Running</span>':""}
        </button>
      `}).join("")}function u(){if(e.fatalError){m.innerHTML=`
      <main class="shell loading-state">
        <section class="hero-card" style="border-color: var(--danger);">
          <h1 style="color: var(--danger);">Initialization Error</h1>
          <p>${i(e.fatalError)}</p>
          <button type="button" onclick="window.location.reload()" style="margin-top: 20px;">Reload Application</button>
        </section>
      </main>
    `;return}if(!e.appState){m.innerHTML=`
      <main class="shell loading-state">
        <section class="hero-card">
          <h1>Cloudflare Tunnel Manager</h1>
          <p>Loading backend state...</p>
        </section>
      </main>
    `;return}const t=e.appState;S(t);const n=g();e.selectedProjectId=n?.id??null;const r=t.status,o=n?I(n,t):"",a=t.settings.projects.length>0,s=t.cloudflaredDetected,p=s,f=n?x(n):null,b=!!(n&&f&&s&&(n.shareMode==="auto"||n.shareMode==="quick"||n.shareMode==="host-html")),H=n?b:s&&a;n?.projectPath.trim()&&!L(n?.projectPath??"")&&t.buildCommandDetected&&t.buildRunning,n?.localHost.trim()&&n?.shareMode!=="host-html"&&n?.shareMode;const U=s?"Installed":"Not installed",q=s?"pill-success":"pill-outline",N=s?"cloudflared was detected and is ready to create a tunnel URL.":"Install cloudflared first before sharing projects.",T=p?"Available":"Install required",E=p?"pill-success":"pill-outline",W=p?"cloudflared is available for tunnel sharing.":"Install cloudflared first before using tunnel sharing.",B=t.cloudflaredPath||"cloudflared.exe (PATH)",R=s?a?"":"Create a project first":"";m.innerHTML=`
    <main class="shell">
      <aside class="sidebar">
        <div class="sidebar-header">
          <div class="logo">
            <img src="./logo.png" alt="App logo" />
            <div class="logo-text">
              <h1>Tunnel Manager</h1>
              <div class="status-indicator tone-${te(r)}">
                <span class="status-dot"></span>
                <span>${i(r.running?"Running":"Stopped")}</span>
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
          <button type="button" class="nav-item ${e.activeTab==="settings"?"active":""}" data-action="tab-settings">
            <svg width="20" height="20" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">
              <circle cx="10" cy="10" r="3" stroke="currentColor" stroke-width="2"/>
              <path d="M10 2V4M10 16V18M18 10H16M4 10H2M15.66 4.34L14.24 5.76M5.76 14.24L4.34 15.66M15.66 15.66L14.24 14.24M5.76 5.76L4.34 4.34" stroke="currentColor" stroke-width="2" stroke-linecap="round"/>
            </svg>
            <span>Settings</span>
          </button>
          <button type="button" class="nav-item ${e.activeTab==="about"?"active":""}" data-action="tab-about">
            <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <circle cx="12" cy="12" r="10"></circle>
              <line x1="12" y1="16" x2="12" y2="12"></line>
              <line x1="12" y1="8" x2="12.01" y2="8"></line>
            </svg>
            <span>About</span>
          </button>
        </nav>

        ${e.activeTab==="overview"?`
          <div class="sidebar-section sidebar-create-section">
            <div class="section-header">
              <h3>Projects</h3>
              <button type="button" class="add-button" data-action="new-project" title="New Project">
                <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <line x1="12" y1="5" x2="12" y2="19"></line>
                  <line x1="5" y1="12" x2="19" y2="12"></line>
                </svg>
              </button>
            </div>
          </div>

          <div class="sidebar-section sidebar-projects-section">
            <div class="project-list">${ne(t)}</div>
          </div>
        `:'<div class="sidebar-section sidebar-projects-empty"></div>'}

        <div class="sidebar-footer">
          <div class="nav-item ${e.activeTab==="logs"?"active":""} logs-nav-item" data-action="tab-logs">
            <span>Logs</span>
          </div>
        </div>
      </aside>

      <section class="main-content">
        ${s?"":`
          <section class="install-banner" data-action="tab-settings">
            <div class="install-banner-copy">
              <span class="install-banner-label">Required setup</span>
              <strong>Install cloudflared first</strong>
              <p>This app will create a Cloudflare Tunnel URL for the selected local project.</p>
              <span class="install-banner-path">Expected path: ${i(B)}</span>
            </div>
            <div class="install-banner-action">
              <button type="button" class="danger-button" data-action="tab-settings">Open Settings</button>
            </div>
          </section>
        `}

        <header class="content-header">
          <div class="header-info">
            <h2>${e.activeTab==="overview"?"Projects":e.activeTab==="settings"?"Settings":e.activeTab==="logs"?"Logs":"About"}</h2>
            ${e.busy?`<p class="busy-indicator">${i(e.busy)}</p>`:""}
          </div>
          <div class="header-actions">
            <button type="button" data-action="start-tunnel" ${e.appState?.status.running||!H?"disabled":""}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"></polygon></svg>
              ${i(f?.label||"Start Tunnel")}
            </button>
            <button type="button" class="secondary" data-action="stop-tunnel">
               <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="6" width="12" height="12"></rect></svg>
               Stop
            </button>
            <button type="button" class="secondary" data-action="refresh" title="Refresh state">
               <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polyline points="23 4 23 10 17 10"></polyline><polyline points="1 20 1 14 7 14"></polyline><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
            </button>
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
                  <label class="wide">Display name<input name="displayName" value="${i(e.editorProject.displayName)}" /></label>
                  <label class="wide">
                    ${e.editorProject.shareMode==="quick"?"Project folder":"Project folder (optional when Local URL is set)"}
                    <div class="folder-picker">
                      <input
                        name="projectPath"
                        value="${i(e.editorProject.projectPath)}"
                        placeholder="${e.editorProject.shareMode==="quick"?"D:\\code\\hr-system":"D:\\code\\site"}"
                      />
                      <button type="button" class="secondary browse-button" data-action="browse-project-folder">Browse</button>
                    </div>
                  </label>
                  <label>
                    Project Type
                    <select name="shareMode">
                      <option value="auto" ${e.editorProject.shareMode==="auto"?"selected":""}>Auto Detect Project</option>
                      <option value="quick" ${e.editorProject.shareMode==="quick"?"selected":""}>Laravel Project (Local Server)</option>
                      <option value="host-html" ${e.editorProject.shareMode==="host-html"?"selected":""}>HTML Project (Folder or Local URL)</option>
                    </select>
                  </label>
                  ${e.editorProject.shareMode==="quick"||e.editorProject.shareMode==="auto"?`<label>${e.editorProject.shareMode==="auto"?"Local host (optional)":"Local host"}<input name="localHost" value="${i(e.editorProject.localHost)}" placeholder="hr-system.test" /></label>`:""}
                  ${e.editorProject.shareMode==="auto"||e.editorProject.shareMode==="host-html"?`<label>Local URL (optional)<input name="localURL" value="${i(e.editorProject.localURL)}" placeholder="http://127.0.0.1:5500" /></label>`:""}
                  ${e.editorProject.shareMode==="auto"?`<label class="wide">Start command (optional)<input name="startCommand" value="${i(e.editorProject.startCommand)}" placeholder="npm run dev -- --port 4173" /></label>`:""}
                   ${e.editorProject.shareMode==="host-html"?`
                        <div class="field-hint hint-info">
                          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                           <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                           <path d="M7 10V7M7 4H7.01" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                         </svg>
                         <span>Share either a local HTML folder or an already running local URL like http://127.0.0.1:5500 via Cloudflare Tunnel.</span>
                        </div>
                        `:e.editorProject.shareMode==="auto"?`
                        <div class="field-hint hint-info">
                          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                           <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                           <path d="M7 10V7M7 4H7.01" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                         </svg>
                         <span>Auto mode can use a local URL, run a start command and detect the dev server, use a Laravel host, or serve a static folder/build output.</span>
                        </div>
                        `:`
                        <div class="field-hint hint-success">
                          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                           <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                           <path d="M4.5 7L6 8.5L9.5 5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                         </svg>
                         <span>Expose your local Laravel Herd hostname securely to a public URL.</span>
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
                      <h2>${i(n?.displayName||"No project selected")}</h2>
                    </div>
                    <div class="selected-header-actions">
                      ${n?`
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

                  ${n?`
                        <div class="hero-project">
                          <div class="hero-project-main">
                            <strong>${i(Y(n))}</strong>
                            <div class="inline-url-row">
                              <p>${i(o||"No public URL is available yet")}</p>
                              ${o?`<button type="button" class="secondary inline-copy-button" data-action="open-url" aria-label="Open public URL">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
                                Open
                              </button>`:""}
                              ${!o&&n.shareMode!=="stable"?`<button type="button" class="secondary inline-copy-button" data-action="regenerate-url" aria-label="Generate public URL" ${s?"":"disabled"}>
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M23 4v6h-6"></path><path d="M1 20v-6h6"></path><path d="M3.51 9a9 9 0 0 1 14.85-3.36L23 10M1 14l4.64 4.36A9 9 0 0 0 20.49 15"></path></svg>
                                Refresh URL
                              </button>`:""}
                            </div>
                          </div>
                        </div>

                      `:'<p class="empty-copy">Create or select a project to start sharing.</p>'}
                </article>
              `:e.activeTab==="settings"?`
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
                            <strong>${i(U)} <span class="pill ${q}">${i(U)}</span></strong>
                            <p>${i(N)}</p>
                          </div>
                          <div class="action-row">
                            ${s?'<button type="button" class="secondary" data-action="ensure-cloudflared">Recheck cloudflared</button>':`
                                  <button type="button" data-action="install-cloudflared">Install cloudflared</button>
                                  <button type="button" class="secondary" data-action="ensure-cloudflared">Check setup</button>
                                `}
                          </div>
                        </div>
                        <div class="metric-card metric-card-split">
                          <div class="metric-card-copy">
                            <span class="summary-label">Setup tunnel</span>
                            <strong>${i(T)} <span class="pill ${E}">${i(T)}</span></strong>
                            <p>${i(W)}</p>
                          </div>
                          <div class="action-row">
                            <button type="button" class="secondary" disabled>Not needed</button>
                          </div>
                        </div>
                      </div>
                    </article>

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
                 `:e.activeTab==="logs"?`
                  <section class="panel logs-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Logs</p>
                        <h2>cloudflared and npm output</h2>
                      </div>
                      <span class="pill pill-outline">${i(`${t.status.lastLogs.length} entries`)}</span>
                    </div>
                    <div class="log-stream">${re(t.status.lastLogs)}</div>
                  </section>
                `:`
                  <article class="panel compact-panel">
                    <div class="panel-header">
                      <div>
                        <p class="eyebrow">Application</p>
                        <h2>Cloudflare Tunnel Manager</h2>
                      </div>
                    </div>
                    <div class="metric-grid">
                      <div class="metric-card">
                         <span class="summary-label">Version</span>
                         <strong>v${i(t.productVersion)}</strong>
                      </div>
                      <div class="metric-card">
                         <span class="summary-label">Platform</span>
                         <strong>Windows</strong>
                      </div>
                    </div>
                    <div style="padding: 24px; color: var(--text-secondary); line-height: 1.6;">
                      <p>Manage your Cloudflare Tunnels easily. This application provides a graphical interface to create and manage tunnel URLs for your local projects using Cloudflare's <code>cloudflared</code> tool.</p>
                      <p style="margin-top: 16px;">Created for developers to simplify the process of sharing local services with the world securely.</p>
                      <p style="margin-top: 16px; color: var(--text-primary); font-weight: 500;">License developed by Reaksmey Kem</p>
                    </div>
                  </article>
               `}

        </section>
      </section>

    </main>
  `,oe()}function v(t,n){return t.elements.namedItem(n)?.value?.trim()??""}function M(){const t=m.querySelector("#project-form");if(!t)return;const n=v(t,"shareMode"),o=["auto","quick","host-html"].includes(n)?n:"auto",a=v(t,"projectPath"),s=v(t,"localURL"),p=v(t,"startCommand"),f=o!=="host-html"&&!L(a),b=v(t,"localHost")||(f?O(a):"");e.editorProject={id:v(t,"id"),displayName:v(t,"displayName"),localHost:b,projectPath:a,localURL:s,startCommand:p,subdomain:v(t,"subdomain"),publicURL:e.editorProject.publicURL,shareMode:o}}async function l(t,n){try{y(t);const r=await n();return e.notice=null,r}catch(r){const o=r instanceof Error?r.message:String(r);d("error",o);return}finally{y(null)}}async function j(t){return!e.appState?.status.running||!e.activeProjectId||e.activeProjectId===t?!0:window.confirm("Another project is currently running. Stop the current project and continue with this one?")}function oe(){const t=m.querySelector("#license-form");t?.addEventListener("submit",async o=>{o.preventDefault();const a=v(t,"licenseToken");e.licenseDraft=a;const s=await l("Activating license...",()=>c.activateLicense(a));s&&(e.appState=s,e.licenseDraft="",s.license.isAdmin&&e.activeTab!=="settings"&&(e.activeTab="settings"),d("success",s.license.message||"License activated"))});const n=m.querySelector("#settings-form");n?.addEventListener("submit",async o=>{o.preventDefault();const a={...e.appState.settings,cloudflaredPath:v(n,"cloudflaredPath"),defaultServiceURL:v(n,"defaultServiceURL")},s=await l("Saving settings...",()=>c.saveSettings(a));s&&(e.appState=s,d("success","Settings saved"))});const r=m.querySelector("#project-form");r?.addEventListener("change",o=>{const a=o.target;a&&(a.name==="shareMode"||a.name==="projectPath"||a.name==="localHost"||a.name==="localURL"||a.name==="startCommand")&&(M(),u())}),r?.addEventListener("submit",async o=>{o.preventDefault();const a={id:v(r,"id"),displayName:v(r,"displayName"),localHost:v(r,"localHost"),projectPath:v(r,"projectPath"),localURL:v(r,"localURL"),startCommand:v(r,"startCommand"),subdomain:v(r,"subdomain"),publicURL:e.editorProject.publicURL,shareMode:e.editorProject.shareMode},s=await l("Saving project...",()=>c.saveProject(a));if(!s)return;e.appState=s;const p=a.id||(s.settings.projects[s.settings.projects.length-1]?.id??null);if(e.selectedProjectId=p,e.editorOpen=!1,!p){d("success","Project preset saved");return}if(!await j(p)){d("success","Project preset saved");return}if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==p){const b=await l("Stopping current project...",()=>c.stopTunnel());if(!b)return;e.appState=b,e.activeProjectId=null}const f=await l("Creating Cloudflare tunnel URL...",()=>c.startQuickTunnel(p));if(f){e.appState=f,e.activeProjectId=p,d("success","Project saved and shared successfully");return}d("success","Project preset saved")})}async function A(t,n){switch(t){case"tab-overview":e.activeTab="overview",e.projectMenuOpen=!1,u();return;case"tab-settings":e.activeTab="settings",e.projectMenuOpen=!1,u();return;case"tab-logs":e.activeTab="logs",e.projectMenuOpen=!1,u();return;case"tab-about":e.activeTab="about",e.projectMenuOpen=!1,u();return;case"toggle-project-menu":e.projectMenuOpen=!e.projectMenuOpen,u();return;case"select-project":e.selectedProjectId=n??null,e.projectMenuOpen=!1,u();return;case"new-project":e.editorOpen=!0,e.editorMode="create",e.editorProject=$(),u();return;case"edit-project":g()&&(e.editorOpen=!0,e.editorMode="edit",e.editorProject={...g()},e.projectMenuOpen=!1,u());return;case"close-editor":e.editorOpen=!1,u();return;case"clear-license":{const r=await l("Removing license...",()=>c.clearLicense());r&&(e.appState=r,e.licenseDraft="",e.activeTab="settings",d("success","License removed"));return}case"browse-project-folder":{M();const r=await l("Opening folder picker...",()=>c.browseProjectFolder(e.editorProject.projectPath));if(typeof r=="string"&&r){const o=O(r);e.editorProject.projectPath=r,(e.editorProject.shareMode==="quick"||e.editorProject.shareMode==="auto")&&!e.editorProject.localHost.trim()&&(e.editorProject.localHost=o),u()}return}case"random-subdomain":{M(),e.editorProject.subdomain=ee(),u();return}case"copy-url":{e.projectMenuOpen=!1;const r=g();if(!r||!e.appState)return;const o=I(r,e.appState);if(!o){d("error","No public URL is available for the selected project");return}await navigator.clipboard.writeText(o),e.copiedProjectId=r.id,P!==null&&window.clearTimeout(P),P=window.setTimeout(()=>{e.copiedProjectId=null,P=null,u()},1800),u();return}}if(!n&&["share-project","share-random","share-quick","regenerate-url","open-url","npm-build","test-project","delete-project"].includes(t)){d("error","Select a project first");return}switch(t){case"start-tunnel":{const r=g();if(r){const a=x(r);await A(a.action,r.id);return}const o=await l("Checking tunnel state...",()=>c.refreshState());if(!o)return;if(e.appState=o,o.status.running){d("info","A tunnel is already running. Stop it first before starting again.");return}try{y("Starting tunnel...");const a=await c.startTunnel();e.appState=a,d("success","Named tunnel started")}catch(a){const s=a instanceof Error?a.message:String(a);if(s.toLowerCase().includes("already running")){const p=await c.refreshState().catch(()=>null);p&&(e.appState=p),d("info","A tunnel is already running. Stop it first before starting again.")}else d("error",s)}finally{y(null)}return}case"ensure-cloudflared":{const r=await l("Checking cloudflared setup...",()=>c.ensureCloudflared());r&&(e.appState=r,d("success","cloudflared is ready to create public URLs."));return}case"install-cloudflared":{const r=await l("Installing cloudflared...",()=>c.installCloudflared());r&&(e.appState=r,d("success","cloudflared installed successfully."));return}case"stop-tunnel":{const r=await l("Stopping tunnel...",()=>c.stopTunnel());r&&(e.appState=r,e.activeProjectId=null,d("success","Tunnel stopped"));return}case"create-tunnel":{const r=await l("Ensuring named tunnel...",()=>c.createTunnel());r&&(e.appState=r,d("success","Named tunnel is ready"));return}case"open-config":await l("Opening config file...",()=>c.openConfigFile());return;case"open-settings":await l("Opening settings file...",()=>c.openSettingsFile());return;case"refresh":{const r=await l("Refreshing state...",()=>c.refreshState());r&&(e.appState=r,!e.activeProjectId&&r.status.running&&(e.activeProjectId=k(r)),d("success","State refreshed"));return}case"share-project":{if(!await j(n))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==n){const o=await l("Stopping current project...",()=>c.stopTunnel());if(!o)return;e.appState=o,e.activeProjectId=null}const r=await l("Sharing project...",()=>c.shareProject(n));r&&(e.appState=r,e.activeProjectId=n,(r.status.activeUrl||r.status.quickUrl)&&(e.projectUrls[n]=r.status.activeUrl||r.status.quickUrl),d("success","Project shared through named tunnel"));return}case"share-random":{if(!await j(n))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==n){const o=await l("Stopping current project...",()=>c.stopTunnel());if(!o)return;e.appState=o,e.activeProjectId=null}const r=await l("Generating random hostname...",()=>c.shareProjectWithRandomURL(n));if(r){if(e.appState=r,e.activeProjectId=n,r.settings.projects){const o=r.settings.projects.find(a=>a.id===n);o?.publicURL&&(e.projectUrls[n]=o.publicURL)}d("success","Random domain share is active")}return}case"share-quick":{if(!await j(n))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==n){const o=await l("Stopping current project...",()=>c.stopTunnel());if(!o)return;e.appState=o,e.activeProjectId=null}const r=await l("Creating Cloudflare tunnel URL...",()=>c.startQuickTunnel(n));r&&(e.appState=r,e.activeProjectId=n,(r.status.activeUrl||r.status.quickUrl)&&(e.projectUrls[n]=r.status.activeUrl||r.status.quickUrl),d("success","Cloudflare tunnel URL is live"));return}case"regenerate-url":{const r=g();if(!r||!await j(r.id))return;if(e.appState?.status.running){const a=await l("Stopping current project...",()=>c.stopTunnel());if(!a)return;e.appState=a,e.activeProjectId=null}const o=await l("Generating new Cloudflare tunnel URL...",()=>c.startQuickTunnel(r.id));if(o){if(e.appState=o,e.activeProjectId=r.id,(o.status.activeUrl||o.status.quickUrl)&&(e.projectUrls[r.id]=o.status.activeUrl||o.status.quickUrl),o.settings.projects){const a=o.settings.projects.find(s=>s.id===r.id);a?.publicURL&&(e.projectUrls[r.id]=a.publicURL)}d("success","New public URL generated")}return}case"open-url":await l("Opening public URL...",()=>c.openPublicURL(n));return;case"npm-build":{await l("Running npm build...",()=>c.runNpmBuild(n))!==void 0&&d("success","npm build started. Watch the log stream for progress");return}case"test-project":{const r=await l("Testing local URL...",()=>c.testProject(n));typeof r=="string"&&d("success",r);return}case"delete-project":if(window.confirm("Delete this project preset?")){const r=await l("Deleting project...",()=>c.deleteProject(n));r&&(e.appState=r,e.selectedProjectId=r.settings.projects[0]?.id??null,e.projectMenuOpen=!1,d("success","Project deleted"))}return}}m.addEventListener("click",t=>{const n=t.target,r=n?.closest("[data-action]"),o=r?.dataset.action,a=n?.closest(".dropdown");if(e.projectMenuOpen&&!a&&(e.projectMenuOpen=!1,u()),!r)return;t.preventDefault();const s=r.dataset.id??g()?.id??null;o&&A(o,s)});async function ae(){u(),await new Promise(n=>setTimeout(n,100));try{await z()&&_(),V(K,Z),F(G,Q),D()}catch{}const t=await l("Loading app state...",()=>c.bootstrap());if(!t){e.fatalError="Failed to load initial application state. Please ensure the backend is running.",u();return}e.appState=t,S(t),e.selectedProjectId=t.settings.projects[0]?.id??null,e.activeProjectId=k(t),u(),window.runtime?.EventsOn("log",n=>{if(!e.appState)return;const r=n;e.appState.status.lastLogs=[...e.appState.status.lastLogs,r].slice(-300),u()}),window.runtime?.EventsOn("status",n=>{if(!e.appState)return;const r=n;if(e.appState.status=r,!e.activeProjectId&&r.running&&(e.activeProjectId=k(e.appState)),e.activeProjectId&&(r.activeUrl||r.quickUrl)){const o=r.activeUrl||r.quickUrl,a=e.projectUrls[e.activeProjectId];if(e.projectUrls[e.activeProjectId]=o,o&&o!==a){const s=e.appState.settings.projects.find(p=>p.id===e.activeProjectId);if(s&&s.publicURL!==o){const p={...s,publicURL:o};c.saveProject(p).then(f=>{e.appState=f,S(f),u()}).catch(()=>{})}}}u()})}window.addEventListener("error",t=>{console.error("Unhandled error:",t.error),!e.appState&&!e.fatalError&&(e.fatalError=`A runtime error occurred during startup: ${t.message}`,u())});ae();
