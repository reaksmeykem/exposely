(function(){const r=document.createElement("link").relList;if(r&&r.supports&&r.supports("modulepreload"))return;for(const s of document.querySelectorAll('link[rel="modulepreload"]'))a(s);new MutationObserver(s=>{for(const o of s)if(o.type==="childList")for(const p of o.addedNodes)p.tagName==="LINK"&&p.rel==="modulepreload"&&a(p)}).observe(document,{childList:!0,subtree:!0});function n(s){const o={};return s.integrity&&(o.integrity=s.integrity),s.referrerPolicy&&(o.referrerPolicy=s.referrerPolicy),s.crossOrigin==="use-credentials"?o.credentials="include":s.crossOrigin==="anonymous"?o.credentials="omit":o.credentials="same-origin",o}function a(s){if(s.ep)return;s.ep=!0;const o=n(s);fetch(s.href,o)}})();function f(){const t=window.go?.main?.App;if(!t)throw!!window.wails||!!window.runtime?new Error("Wails bindings are not yet available. This might happen if Bootstrap is called too early."):new Error("Wails bindings are not available. Please ensure you are running the application via Wails.");return t}const c={bootstrap:()=>f().Bootstrap(),refreshState:()=>f().RefreshState(),ensureCloudflared:()=>f().EnsureCloudflared(),installCloudflared:()=>f().InstallCloudflared(),saveSettings:t=>f().SaveSettings(t),saveProject:t=>f().SaveProject(t),deleteProject:t=>f().DeleteProject(t),shareProject:t=>f().ShareProject(t),shareProjectWithRandomURL:t=>f().ShareProjectWithRandomURL(t),startQuickTunnel:t=>f().StartQuickTunnel(t),startTunnel:()=>f().StartTunnel(),stopTunnel:()=>f().StopTunnel(),createTunnel:()=>f().CreateTunnel(),activateLicense:t=>f().ActivateLicense(t),clearLicense:()=>f().ClearLicense(),runNpmBuild:t=>f().RunNpmBuild(t),openPublicURL:t=>f().OpenPublicURL(t),openConfigFile:()=>f().OpenConfigFile(),openSettingsFile:()=>f().OpenSettingsFile(),browseProjectFolder:t=>f().BrowseProjectFolder(t),testProject:t=>f().TestProject(t)};function B(){window.runtime.WindowCenter()}function D(t,r){window.runtime.WindowSetSize(t,r)}function F(t,r){window.runtime.WindowSetMinSize(t,r)}function _(){window.runtime.WindowUnmaximise()}function V(){return window.runtime.WindowIsMaximised()}const R=()=>({id:"",displayName:"",localHost:"",subdomain:"",publicURL:"",projectPath:"",shareMode:"quick"}),e={appState:null,fatalError:null,selectedProjectId:null,activeProjectId:null,projectUrls:{},copiedProjectId:null,activeTab:"overview",projectMenuOpen:!1,editorOpen:!1,editorMode:"create",editorProject:R(),notice:null,busy:null,licenseDraft:""},I=document.querySelector("#app");if(!I)throw new Error("App root not found");const b=I,z=1040,G=700,Q=900,K=620;let w=null,y=null;function i(t){return t.replaceAll("&","&amp;").replaceAll("<","&lt;").replaceAll(">","&gt;").replaceAll('"',"&quot;").replaceAll("'","&#039;")}function Z(t,r){return t.publicURL?t.publicURL:t.subdomain?`https://${t.subdomain}.${r}`:""}function $(t,r){const n=e.activeProjectId===t.id?(r.status.activeUrl||r.status.quickUrl||"").trim():"";if(n)return n;const a=e.projectUrls[t.id]?.trim()||"";return a||Z(t,r.settings.defaultDomain).trim()}function C(t){return t.shareMode==="quick"?{action:"share-quick",label:"Create Public URL"}:{action:"share-quick",label:"Create Public URL"}}function x(t){const r=t.trim().replace(/[/\\]+$/,"");if(!r)return"";const n=r.split(/[/\\]+/),a=n[n.length-1]?.trim()??"";if(!a)return"";const s=a.toLowerCase().replace(/[^a-z0-9]+/g,"-").replace(/^-+|-+$/g,"");return s?`${s}.test`:""}function J(){return Math.random().toString(36).slice(2,8)}function h(){const t=e.appState?.settings.projects??[];return t.length===0?null:e.selectedProjectId?t.find(r=>r.id===e.selectedProjectId)??t[0]:t[0]}function S(t){for(const r of t.settings.projects)r.publicURL?.trim()&&(e.projectUrls[r.id]=r.publicURL.trim())}function L(t){const r=(t.status.activeUrl||t.status.quickUrl||"").trim();if(!t.status.running)return null;if(r){const n=t.settings.projects.find(a=>(e.projectUrls[a.id]||a.publicURL||"").trim()===r);if(n)return n.id}return e.selectedProjectId||t.settings.projects[0]?.id||null}function d(t,r){e.notice={tone:t,message:r},w!==null&&window.clearTimeout(w),w=window.setTimeout(()=>{e.notice=null,w=null,u()},3e3),u()}function P(t){e.busy=t,u()}function X(t){return t.running?"success":t.lastError?"error":"idle"}function Y(t){return t.slice(-12).reverse().map(r=>`
        <div class="log-entry log-${i(r.level)}">
          <span>${i(new Date(r.timestamp).toLocaleTimeString())}</span>
          <strong>${i(r.source)}</strong>
          <p>${i(r.message)}</p>
        </div>
      `).join("")}function ee(t){return t.settings.projects.map(r=>{const n=h()?.id===r.id,a=e.activeProjectId===r.id&&t.status.running;return`
        <button type="button" class="project-row ${n?"selected":""}" data-action="select-project" data-id="${i(r.id)}">
          <div class="project-copy">
            <strong>${i(r.displayName)}</strong>
          </div>
          ${a?'<span class="project-running-badge">Running</span>':""}
        </button>
      `}).join("")}function u(){if(e.fatalError){b.innerHTML=`
      <main class="shell loading-state">
        <section class="hero-card" style="border-color: var(--danger);">
          <h1 style="color: var(--danger);">Initialization Error</h1>
          <p>${i(e.fatalError)}</p>
          <button type="button" onclick="window.location.reload()" style="margin-top: 20px;">Reload Application</button>
        </section>
      </main>
    `;return}if(!e.appState){b.innerHTML=`
      <main class="shell loading-state">
        <section class="hero-card">
          <h1>Cloudflare Tunnel Manager</h1>
          <p>Loading backend state...</p>
        </section>
      </main>
    `;return}const t=e.appState;S(t);const r=h();e.selectedProjectId=r?.id??null;const n=t.status,a=r?$(r,t):"",s=t.settings.projects.length>0,o=t.cloudflaredDetected,p=o,g=r?C(r):null,j=!!(r&&g&&o&&r.shareMode==="quick"),A=r?j:o&&s;r?.projectPath.trim()&&t.buildCommandDetected&&t.buildRunning,r?.localHost.trim();const U=o?"Installed":"Not installed",N=o?"pill-success":"pill-outline",E=o?"cloudflared was detected and is ready to create a tunnel URL.":"Install cloudflared first before sharing projects.",T=p?"Available":"Install required",q=p?"pill-success":"pill-outline",W=p?"cloudflared is available for tunnel sharing.":"Install cloudflared first before using tunnel sharing.",H=t.cloudflaredPath||"cloudflared.exe (PATH)",M=o?s?"":"Create a project first":"";b.innerHTML=`
    <main class="shell">
      <aside class="sidebar">
        <div class="sidebar-header">
          <div class="logo">
            <img src="./logo.png" alt="App logo" />
            <div class="logo-text">
              <h1>Tunnel Manager</h1>
              <div class="status-indicator tone-${X(n)}">
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
            <div class="project-list">${ee(t)}</div>
          </div>
        `:'<div class="sidebar-section sidebar-projects-empty"></div>'}

        <div class="sidebar-footer">
          <div class="nav-item ${e.activeTab==="logs"?"active":""} logs-nav-item" data-action="tab-logs">
            <span>Logs</span>
          </div>
        </div>
      </aside>

      <section class="main-content">
        ${o?"":`
          <section class="install-banner" data-action="tab-settings">
            <div class="install-banner-copy">
              <span class="install-banner-label">Required setup</span>
              <strong>Install cloudflared first</strong>
              <p>This app will create a Cloudflare Tunnel URL for the selected local project.</p>
              <span class="install-banner-path">Expected path: ${i(H)}</span>
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
            <button type="button" data-action="start-tunnel" ${e.appState?.status.running||!A?"disabled":""}>
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="5 3 19 12 5 21 5 3"></polygon></svg>
              ${i(g?.label||"Start Tunnel")}
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

        ${M?`<p class="header-hint">${i(M)}</p>`:""}

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
                    Project folder
                    <div class="folder-picker">
                      <input name="projectPath" value="${i(e.editorProject.projectPath)}" placeholder="D:\\code\\hr-system" />
                      <button type="button" class="secondary browse-button" data-action="browse-project-folder">Browse</button>
                    </div>
                  </label>
                  <label>Local host<input name="localHost" value="${i(e.editorProject.localHost)}" placeholder="hr-system.test" /></label>
                  <label>
                    Share mode
                    <input name="shareModeLabel" value="Cloudflare Tunnel URL" disabled />
                    <input type="hidden" name="shareMode" value="quick" />
                  </label>
                   
                       <div class="field-hint hint-success">
                         <svg width="14" height="14" viewBox="0 0 14 14" fill="none" xmlns="http://www.w3.org/2000/svg">
                          <circle cx="7" cy="7" r="6" stroke="currentColor" stroke-width="1.2"/>
                          <path d="M4.5 7L6 8.5L9.5 5" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" stroke-linejoin="round"/>
                        </svg>
                        <span>This mode generates a Cloudflare public tunnel URL without asking for a custom domain.</span>
                      </div>
                    
                  <div class="action-row wide"><button type="submit">${e.editorMode==="create"?"Save Project":"Update Project"}</button></div>
                </form>
              </section>
            `:""}

          ${e.activeTab==="overview"?`
                <article class="panel selected-panel">
                  <div class="panel-header">
                    <div>
                      <h2>${i(r?.displayName||"No project selected")}</h2>
                    </div>
                    <div class="selected-header-actions">
                      ${r?`
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

                  ${r?`
                        <div class="hero-project">
                          <div class="hero-project-main">
                            <strong>${i(r.localHost)}</strong>
                            <div class="inline-url-row">
                              <p>${i(a||"No public URL is available yet")}</p>
                              ${a?`<button type="button" class="secondary inline-copy-button" data-action="open-url" aria-label="Open public URL">
                                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
                                Open
                              </button>`:""}
                              ${!a&&r.shareMode!=="stable"?`<button type="button" class="secondary inline-copy-button" data-action="regenerate-url" aria-label="Generate public URL" ${o?"":"disabled"}>
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
                            <strong>${i(U)} <span class="pill ${N}">${i(U)}</span></strong>
                            <p>${i(E)}</p>
                          </div>
                          <div class="action-row">
                            ${o?'<button type="button" class="secondary" data-action="ensure-cloudflared">Recheck cloudflared</button>':`
                                  <button type="button" data-action="install-cloudflared">Install cloudflared</button>
                                  <button type="button" class="secondary" data-action="ensure-cloudflared">Check setup</button>
                                `}
                          </div>
                        </div>
                        <div class="metric-card metric-card-split">
                          <div class="metric-card-copy">
                            <span class="summary-label">Setup tunnel</span>
                            <strong>${i(T)} <span class="pill ${q}">${i(T)}</span></strong>
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
                    <div class="log-stream">${Y(t.status.lastLogs)}</div>
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
  `,te()}function v(t,r){return t.elements.namedItem(r)?.value?.trim()??""}function k(){const t=b.querySelector("#project-form");if(!t)return;const r=v(t,"shareMode"),a=["quick"].includes(r)?r:"quick",s=v(t,"projectPath"),o=v(t,"localHost")||x(s);e.editorProject={id:v(t,"id"),displayName:v(t,"displayName"),localHost:o,projectPath:s,subdomain:v(t,"subdomain"),publicURL:e.editorProject.publicURL,shareMode:a}}async function l(t,r){try{P(t);const n=await r();return e.notice=null,n}catch(n){const a=n instanceof Error?n.message:String(n);d("error",a);return}finally{P(null)}}async function m(t){return!e.appState?.status.running||!e.activeProjectId||e.activeProjectId===t?!0:window.confirm("Another project is currently running. Stop the current project and continue with this one?")}function te(){const t=b.querySelector("#license-form");t?.addEventListener("submit",async a=>{a.preventDefault();const s=v(t,"licenseToken");e.licenseDraft=s;const o=await l("Activating license...",()=>c.activateLicense(s));o&&(e.appState=o,e.licenseDraft="",o.license.isAdmin&&e.activeTab!=="settings"&&(e.activeTab="settings"),d("success",o.license.message||"License activated"))});const r=b.querySelector("#settings-form");r?.addEventListener("submit",async a=>{a.preventDefault();const s={...e.appState.settings,cloudflaredPath:v(r,"cloudflaredPath"),defaultServiceURL:v(r,"defaultServiceURL")},o=await l("Saving settings...",()=>c.saveSettings(s));o&&(e.appState=o,d("success","Settings saved"))});const n=b.querySelector("#project-form");n?.addEventListener("change",a=>{const s=a.target;s&&(s.name==="shareMode"||s.name==="projectPath"||s.name==="localHost")&&(k(),u())}),n?.addEventListener("submit",async a=>{a.preventDefault();const s={id:v(n,"id"),displayName:v(n,"displayName"),localHost:v(n,"localHost"),projectPath:v(n,"projectPath"),subdomain:v(n,"subdomain"),publicURL:e.editorProject.publicURL,shareMode:"quick"},o=await l("Saving project...",()=>c.saveProject(s));if(!o)return;e.appState=o;const p=s.id||(o.settings.projects[o.settings.projects.length-1]?.id??null);if(e.selectedProjectId=p,e.editorOpen=!1,!p){d("success","Project preset saved");return}if(!await m(p)){d("success","Project preset saved");return}if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==p){const j=await l("Stopping current project...",()=>c.stopTunnel());if(!j)return;e.appState=j,e.activeProjectId=null}s.shareMode;const g=await l("Creating Cloudflare tunnel URL...",()=>c.startQuickTunnel(p));if(g){e.appState=g,e.activeProjectId=p,d("success","Project saved and shared successfully");return}d("success","Project preset saved")})}async function O(t,r){switch(t){case"tab-overview":e.activeTab="overview",e.projectMenuOpen=!1,u();return;case"tab-settings":e.activeTab="settings",e.projectMenuOpen=!1,u();return;case"tab-logs":e.activeTab="logs",e.projectMenuOpen=!1,u();return;case"tab-about":e.activeTab="about",e.projectMenuOpen=!1,u();return;case"toggle-project-menu":e.projectMenuOpen=!e.projectMenuOpen,u();return;case"select-project":e.selectedProjectId=r??null,e.projectMenuOpen=!1,u();return;case"new-project":e.editorOpen=!0,e.editorMode="create",e.editorProject=R(),u();return;case"edit-project":h()&&(e.editorOpen=!0,e.editorMode="edit",e.editorProject={...h()},e.projectMenuOpen=!1,u());return;case"close-editor":e.editorOpen=!1,u();return;case"clear-license":{const n=await l("Removing license...",()=>c.clearLicense());n&&(e.appState=n,e.licenseDraft="",e.activeTab="settings",d("success","License removed"));return}case"browse-project-folder":{k();const n=await l("Opening folder picker...",()=>c.browseProjectFolder(e.editorProject.projectPath));if(typeof n=="string"&&n){const a=x(n);e.editorProject.projectPath=n,e.editorProject.localHost.trim()||(e.editorProject.localHost=a),u()}return}case"random-subdomain":{k(),e.editorProject.subdomain=J(),u();return}case"copy-url":{e.projectMenuOpen=!1;const n=h();if(!n||!e.appState)return;const a=$(n,e.appState);if(!a){d("error","No public URL is available for the selected project");return}await navigator.clipboard.writeText(a),e.copiedProjectId=n.id,y!==null&&window.clearTimeout(y),y=window.setTimeout(()=>{e.copiedProjectId=null,y=null,u()},1800),u();return}}if(!r&&["share-project","share-random","share-quick","regenerate-url","open-url","npm-build","test-project","delete-project"].includes(t)){d("error","Select a project first");return}switch(t){case"start-tunnel":{const n=h();if(n){const s=C(n);await O(s.action,n.id);return}const a=await l("Checking tunnel state...",()=>c.refreshState());if(!a)return;if(e.appState=a,a.status.running){d("info","A tunnel is already running. Stop it first before starting again.");return}try{P("Starting tunnel...");const s=await c.startTunnel();e.appState=s,d("success","Named tunnel started")}catch(s){const o=s instanceof Error?s.message:String(s);if(o.toLowerCase().includes("already running")){const p=await c.refreshState().catch(()=>null);p&&(e.appState=p),d("info","A tunnel is already running. Stop it first before starting again.")}else d("error",o)}finally{P(null)}return}case"ensure-cloudflared":{const n=await l("Checking cloudflared setup...",()=>c.ensureCloudflared());n&&(e.appState=n,d("success","cloudflared is ready to create public URLs."));return}case"install-cloudflared":{const n=await l("Installing cloudflared...",()=>c.installCloudflared());n&&(e.appState=n,d("success","cloudflared installed successfully."));return}case"stop-tunnel":{const n=await l("Stopping tunnel...",()=>c.stopTunnel());n&&(e.appState=n,e.activeProjectId=null,d("success","Tunnel stopped"));return}case"create-tunnel":{const n=await l("Ensuring named tunnel...",()=>c.createTunnel());n&&(e.appState=n,d("success","Named tunnel is ready"));return}case"open-config":await l("Opening config file...",()=>c.openConfigFile());return;case"open-settings":await l("Opening settings file...",()=>c.openSettingsFile());return;case"refresh":{const n=await l("Refreshing state...",()=>c.refreshState());n&&(e.appState=n,!e.activeProjectId&&n.status.running&&(e.activeProjectId=L(n)),d("success","State refreshed"));return}case"share-project":{if(!await m(r))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==r){const a=await l("Stopping current project...",()=>c.stopTunnel());if(!a)return;e.appState=a,e.activeProjectId=null}const n=await l("Sharing project...",()=>c.shareProject(r));n&&(e.appState=n,e.activeProjectId=r,(n.status.activeUrl||n.status.quickUrl)&&(e.projectUrls[r]=n.status.activeUrl||n.status.quickUrl),d("success","Project shared through named tunnel"));return}case"share-random":{if(!await m(r))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==r){const a=await l("Stopping current project...",()=>c.stopTunnel());if(!a)return;e.appState=a,e.activeProjectId=null}const n=await l("Generating random hostname...",()=>c.shareProjectWithRandomURL(r));if(n){if(e.appState=n,e.activeProjectId=r,n.settings.projects){const a=n.settings.projects.find(s=>s.id===r);a?.publicURL&&(e.projectUrls[r]=a.publicURL)}d("success","Random domain share is active")}return}case"share-quick":{if(!await m(r))return;if(e.appState?.status.running&&e.activeProjectId&&e.activeProjectId!==r){const a=await l("Stopping current project...",()=>c.stopTunnel());if(!a)return;e.appState=a,e.activeProjectId=null}const n=await l("Creating Cloudflare tunnel URL...",()=>c.startQuickTunnel(r));n&&(e.appState=n,e.activeProjectId=r,(n.status.activeUrl||n.status.quickUrl)&&(e.projectUrls[r]=n.status.activeUrl||n.status.quickUrl),d("success","Cloudflare tunnel URL is live"));return}case"regenerate-url":{const n=h();if(!n||!await m(n.id))return;if(e.appState?.status.running){const s=await l("Stopping current project...",()=>c.stopTunnel());if(!s)return;e.appState=s,e.activeProjectId=null}const a=await l("Generating new Cloudflare tunnel URL...",()=>c.startQuickTunnel(n.id));if(a){if(e.appState=a,e.activeProjectId=n.id,(a.status.activeUrl||a.status.quickUrl)&&(e.projectUrls[n.id]=a.status.activeUrl||a.status.quickUrl),a.settings.projects){const s=a.settings.projects.find(o=>o.id===n.id);s?.publicURL&&(e.projectUrls[n.id]=s.publicURL)}d("success","New public URL generated")}return}case"open-url":await l("Opening public URL...",()=>c.openPublicURL(r));return;case"npm-build":{await l("Running npm build...",()=>c.runNpmBuild(r))!==void 0&&d("success","npm build started. Watch the log stream for progress");return}case"test-project":{const n=await l("Testing local URL...",()=>c.testProject(r));typeof n=="string"&&d("success",n);return}case"delete-project":if(window.confirm("Delete this project preset?")){const n=await l("Deleting project...",()=>c.deleteProject(r));n&&(e.appState=n,e.selectedProjectId=n.settings.projects[0]?.id??null,e.projectMenuOpen=!1,d("success","Project deleted"))}return}}b.addEventListener("click",t=>{const r=t.target,n=r?.closest("[data-action]"),a=n?.dataset.action,s=r?.closest(".dropdown");if(e.projectMenuOpen&&!s&&(e.projectMenuOpen=!1,u()),!n)return;t.preventDefault();const o=n.dataset.id??h()?.id??null;a&&O(a,o)});async function ne(){u(),await new Promise(r=>setTimeout(r,100));try{await V()&&_(),F(Q,K),D(z,G),B()}catch{}const t=await l("Loading app state...",()=>c.bootstrap());if(!t){e.fatalError="Failed to load initial application state. Please ensure the backend is running.",u();return}e.appState=t,S(t),e.selectedProjectId=t.settings.projects[0]?.id??null,e.activeProjectId=L(t),u(),window.runtime?.EventsOn("log",r=>{if(!e.appState)return;const n=r;e.appState.status.lastLogs=[...e.appState.status.lastLogs,n].slice(-300),u()}),window.runtime?.EventsOn("status",r=>{if(!e.appState)return;const n=r;if(e.appState.status=n,!e.activeProjectId&&n.running&&(e.activeProjectId=L(e.appState)),e.activeProjectId&&(n.activeUrl||n.quickUrl)){const a=n.activeUrl||n.quickUrl,s=e.projectUrls[e.activeProjectId];if(e.projectUrls[e.activeProjectId]=a,a&&a!==s){const o=e.appState.settings.projects.find(p=>p.id===e.activeProjectId);if(o&&o.publicURL!==a){const p={...o,publicURL:a};c.saveProject(p).then(g=>{e.appState=g,S(g),u()}).catch(()=>{})}}}u()})}window.addEventListener("error",t=>{console.error("Unhandled error:",t.error),!e.appState&&!e.fatalError&&(e.fatalError=`A runtime error occurred during startup: ${t.message}`,u())});ne();
