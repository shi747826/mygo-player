// Controls share the video and ordered report queue above. All remote strings
// enter the DOM through textContent; SVG paths below are fixed local assets.
const ui=document.getElementById('player-ui');
let config={title:'',episode:0,episodes:[],currentEpisode:'',qualities:[],quality:'',autoplay:false,portrait:false,pinned:false,fullscreen:false,windowFullscreen:false,canPrevious:false,canNext:false,danmakuAvailable:false,danmakuEnabled:true,danmakuPosition:'full',danmakuSize:'medium',danmakuSpeed:'medium',danmakuOpacity:80};
window.playerConfigure=next=>{if(!next||typeof next!=='object')return false;const before=JSON.stringify(config);for(const key of ['title','currentEpisode','quality'])if(typeof next[key]==='string')config[key]=next[key];for(const key of ['autoplay','portrait','pinned','fullscreen','windowFullscreen','canPrevious','canNext','danmakuEnabled','danmakuAvailable'])if(typeof next[key]==='boolean')config[key]=next[key];
for(const key of ['danmakuPosition','danmakuSize','danmakuSpeed'])if(typeof next[key]==='string')config[key]=next[key];
if(Number.isInteger(next.danmakuOpacity)&&next.danmakuOpacity>=10&&next.danmakuOpacity<=100)config.danmakuOpacity=next.danmakuOpacity;if(Number.isInteger(next.episode)&&next.episode>=0)config.episode=next.episode;if(Array.isArray(next.qualities))config.qualities=next.qualities.filter(x=>typeof x==='string');if(Array.isArray(next.episodes))config.episodes=next.episodes.filter(x=>x&&typeof x.id==='string'&&Number.isInteger(x.number)).map(x=>({id:x.id,number:x.number}));if(typeof renderConfig==='function'&&JSON.stringify(config)!==before)renderConfig();return true;};
// The minimal host script harness deliberately has no UI DOM.
var renderConfig;
if(ui&&typeof ui.addEventListener==='function'){
 const get=id=>document.getElementById(id),menu=get('player-menu'),seek=get('seek'),preview=get('seek-preview');
 let dmPanelHovered=false;
 const icons={window:'M3 3h18a2 2 0 012 2v14a2 2 0 01-2 2H3a2 2 0 01-2-2V5a2 2 0 012-2zm0 5v11h18V8z',windowExit:'M3 3h18v18H3zm2 2v14h14V5zm2 4h10v8H7z',pip:'M3 5h18a2 2 0 012 2v10a2 2 0 01-2 2h-7v-2h7V7H3v4H1V7a2 2 0 012-2zm-1 7h8a2 2 0 012 2v5a2 2 0 01-2 2H2a2 2 0 01-2-2v-5a2 2 0 012-2z',play:'M8 5v14l11-7z',pause:'M6 5h4v14H6zm8 0h4v14h-4z',previous:'M5 5h2v14H5zm14 0v14L8 12z',next:'M17 5h2v14h-2zM5 5l11 7-11 7z',volume:'M3 9v6h4l5 4V5L7 9zm12-1v8a5 5 0 000-8zm0-5v2a8 8 0 010 14v2a10 10 0 000-18z',mute:'M3 9v6h4l5 4V5L7 9zm12-1 3 3 3-3 1 1-3 3 3 3-1 1-3-3-3 3-1-1 3-3-3-3z',fullscreen:'M3 3h7v2H5v5H3zm11 0h7v7h-2V5h-5zM3 14h2v5h5v2H3zm16 0h2v7h-7v-2h5z',exit:'M8 3h2v7H3V8h5zm6 0h2v5h5v2h-7zM3 14h7v7H8v-5H3zm11 0h7v2h-5v5h-2z',settings:'M10 2h4l1 3 3 1 3-1 2 4-2 2v3l2 2-2 4-3-1-3 1-1 3h-4l-1-3-3-1-3 1-2-4 2-2v-3L1 9l2-4 3 1 3-1zm2 6a4 4 0 100 8 4 4 0 000-8z'};
 function icon(id,name){const button=get(id);while(button.firstChild)button.removeChild(button.firstChild);const svg=document.createElementNS('http://www.w3.org/2000/svg','svg'),path=document.createElementNS('http://www.w3.org/2000/svg','path');svg.setAttribute('viewBox','0 0 24 24');svg.setAttribute('aria-hidden','true');path.setAttribute('d',icons[name]);svg.appendChild(path);button.appendChild(svg);}
 let idleTimer=0,opened='',dragging=false,lastVolume=1;
 const menuButtons=['quality','episode','rate','volume-button','settings'];
 const hoverMenus=['quality','episode','rate','settings'];
 function reveal(){ui.classList.remove('is-idle');document.body.classList.remove('player-idle');clearTimeout(idleTimer);if(!video.paused&&!opened&&!dragging)idleTimer=setTimeout(()=>{ui.classList.add('is-idle');document.body.classList.add('player-idle');},3000);}
 async function action(name,value=''){reveal();try{const response=await fetch(endpoint.replace('/state?','/action?'),{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name,value:String(value)}),cache:'no-store'});if(!response.ok)hint.textContent='操作未完成，请重试';}catch(_){hint.textContent='播放器连接中断，请重试';}}
 function closeMenu(){opened='';menu.hidden=true;for(const id of menuButtons)get(id).setAttribute('aria-expanded','false');reveal();}
 function toggle(){window.playerCommand(video.paused?'play':'pause',0);reveal();}
 function full(value){void action('fullscreen',value);}
 function clock(seconds){seconds=Math.max(0,Math.floor(Number.isFinite(seconds)?seconds:0));const h=Math.floor(seconds/3600),m=Math.floor(seconds%3600/60),s=String(seconds%60).padStart(2,'0');return h?h+':'+String(m).padStart(2,'0')+':'+s:String(m).padStart(2,'0')+':'+s;}
 function sync(){icon('toggle-play',video.paused?'play':'pause');get('toggle-play').setAttribute('aria-label',video.paused?'播放':'暂停');get('time').textContent=clock(video.currentTime)+' / '+clock(video.duration);get('rate').textContent=video.playbackRate===1?'倍速':video.playbackRate+'×';icon('volume-button',video.muted||video.volume===0?'mute':'volume');seek.disabled=!Number.isFinite(video.duration)||video.duration<=0;if(!dragging){seek.value=seek.disabled?'0':String(Math.round(video.currentTime/video.duration*1000));seek.style.setProperty('--progress',Number(seek.value)/10+'%');}report();}
 function item(parent,label,selected,callback){const button=document.createElement('button');button.type='button';button.className='menu-item';button.textContent=label;button.setAttribute('role','menuitemradio');button.setAttribute('aria-checked',String(selected));button.addEventListener('click',()=>{callback();closeMenu();});parent.appendChild(button);return button;}
 function drawMenu(){menu.replaceChildren();const title=document.createElement('div');title.className='menu-title';title.textContent={quality:'画质',episode:'选集',rate:'播放速度','volume-button':'音量',settings:'播放设置'}[opened];menu.appendChild(title);
  if(opened==='quality'){if(!config.qualities.length)title.textContent='暂无可选画质';for(const quality of config.qualities)item(menu,quality,quality===config.quality,()=>void action('quality',quality));}
  if(opened==='episode'){
   if(!config.episodes.length)title.textContent='暂无可选分集';
   const grid=document.createElement('div');grid.className='episode-grid';menu.appendChild(grid);
   let current=null;
   for(const episode of config.episodes){
     const button=item(grid,String(episode.number),episode.id===config.currentEpisode,()=>void action('episode',episode.id));
     if(episode.id===config.currentEpisode)current=button;
   }
   // 打开就把正在看的那一集滚到中间，而不是从第 1 集开始。
   if(current&&typeof current.scrollIntoView==='function'){
     const scroll=()=>current.scrollIntoView({block:'center'});
     if(typeof requestAnimationFrame==='function')requestAnimationFrame(scroll);else scroll();
   }
  }
  if(opened==='rate')for(const rate of [.5,.75,1,1.25,1.5,2,3,4])item(menu,rate===1?'正常 · 1×':rate+'×',rate===video.playbackRate,()=>window.playerCommand('rate',rate));
  if(opened==='settings')for(const [label,key,name] of [['自动连播','autoplay','autoplay'],['竖屏模式','portrait','portrait'],['窗口置顶','pinned','pin']])item(menu,(config[key]?'✓  ':'')+label,config[key],()=>void action(name,!config[key]));
 }
 // 弹层对齐到触发按钮：水平居中在按钮上方，并限制在窗口内。
 function anchor(el,button){
   if(!el||!button||typeof button.getBoundingClientRect!=='function')return;
   const apply=()=>{
     const r=button.getBoundingClientRect();
     const w=el.offsetWidth||(el.getBoundingClientRect?el.getBoundingClientRect().width:0)||0;
     const vw=(document.documentElement&&document.documentElement.clientWidth)||window.innerWidth||0;
     if(!w||!vw)return false;
     let left=r.left+r.width/2-w/2;
     left=Math.max(8,Math.min(left,vw-w-8));
     el.style.left=left+'px';
     el.style.right='auto';
     return true;
   };
   // 面板刚从 hidden 变可见时布局可能还没生效，下一帧再对齐一次。
   if(!apply()&&typeof requestAnimationFrame==='function')requestAnimationFrame(apply);
 }
 function openMenu(id){if(opened===id){return;}closeMenu();opened=id;menu.hidden=false;menu.setAttribute('role','menu');get(id).setAttribute('aria-expanded','true');drawMenu();anchor(menu,get(id));reveal();}
 // 悬停展开：移出按钮或菜单后收起，按钮底色保持不变。
 let menuCloseTimer=0,menuHovered=false,overVolumeButton=false,overVolumePanel=false;
 function cancelMenuClose(){clearTimeout(menuCloseTimer);}
 function scheduleMenuClose(){cancelMenuClose();menuCloseTimer=setTimeout(()=>{if(!menuHovered&&!overVolumeButton&&!overVolumePanel)closeMenu();},160);}
 menu.addEventListener('mouseenter',()=>{menuHovered=true;cancelMenuClose();});
 menu.addEventListener('mouseleave',()=>{menuHovered=false;scheduleMenuClose();});
 for(const id of hoverMenus){
   const button=get(id);
   button.addEventListener('mouseenter',()=>{cancelMenuClose();openMenu(id);});
   button.addEventListener('mouseleave',()=>{scheduleMenuClose();});
   button.addEventListener('click',()=>{cancelMenuClose();openMenu(id);});
 }
 // 音量：悬停展开竖向滑杆，点图标直接静音/恢复。
 const volumePanel=get('volume-panel'),volumeSlider=get('volume-slider'),volumeValue=get('volume-value');
 function syncVolumePanel(){const v=video.muted?0:Math.round(video.volume*100);if(volumeValue)volumeValue.textContent=String(v);if(volumeSlider){volumeSlider.value=String(v);volumeSlider.style.setProperty('--vol',v+'%');}}
 function hideVolumePanel(){overVolumeButton=false;overVolumePanel=false;if(volumePanel)volumePanel.hidden=true;}
 // 指针离开按钮或面板后 150 毫秒再收起，方便从按钮移到滑杆上。
 function scheduleVolumeHide(){setTimeout(()=>{if(!overVolumeButton&&!overVolumePanel)hideVolumePanel();},150);}
 get('volume-button').addEventListener('mouseenter',()=>{cancelMenuClose();closeMenu();overVolumeButton=true;if(volumePanel){volumePanel.hidden=false;anchor(volumePanel,get('volume-button'));}syncVolumePanel();reveal();});
 get('volume-button').addEventListener('mouseleave',()=>{overVolumeButton=false;scheduleVolumeHide();});
 get('volume-button').addEventListener('click',()=>{overVolumeButton=true;if(volumePanel)volumePanel.hidden=false;window.playerCommand('volume',video.muted||video.volume===0?lastVolume:0);syncVolumePanel();sync();reveal();});
 if(volumePanel){volumePanel.addEventListener('mouseenter',()=>{overVolumePanel=true;cancelMenuClose();});volumePanel.addEventListener('mouseleave',()=>{overVolumePanel=false;scheduleVolumeHide();});}
 // 滚轮调音量：每格 5%，中间弹出与 B 站一致的白底提示。
 const volumeToast=get('volume-toast'),volumeToastValue=get('volume-toast-value'),volumeToastIcon=get('volume-toast-icon');
 let toastTimer=0;
 function showVolumeToast(){
   if(!volumeToast)return;
   const percent=Math.round((video.muted?0:video.volume)*100);
   if(volumeToastValue)volumeToastValue.textContent=percent+'%';
   if(volumeToastIcon)volumeToastIcon.setAttribute('d',(video.muted||video.volume===0)?icons.mute:icons.volume);
   volumeToast.classList.add('show');
   clearTimeout(toastTimer);
   toastTimer=setTimeout(()=>volumeToast.classList.remove('show'),900);
 }
 function stepsFromWheel(event){
   if(!event||!Number.isFinite(event.deltaY))return 0;
   return event.deltaY<0?1:-1;
 }
 const onWheel=event=>{
   const steps=stepsFromWheel(event);
   if(!steps)return;
   if(event.cancelable)event.preventDefault();
   const current=video.muted?0:video.volume;
   const next=Math.min(1,Math.max(0,Math.round((current+steps*0.05)*100)/100));
   if(next>0)lastVolume=next;
   window.playerCommand('volume',next);
   syncVolumePanel();sync();showVolumeToast();reveal();
 };
 // 只监听画面本身：在选集/倍速等弹层里滚动仍然是滚动列表。
 if(typeof video.addEventListener==='function')video.addEventListener('wheel',onWheel,{passive:false});
 if(volumeSlider)volumeSlider.addEventListener('input',()=>{const value=Number(volumeSlider.value)/100;if(value>0)lastVolume=value;window.playerCommand('volume',value);syncVolumePanel();sync();reveal();});
 // 弹幕设置面板：位置、字号、不透明度和开关都由应用保存在本机。
 const dmPanel=get('danmaku-panel'),dmButton=get('danmaku-button'),dmLayer=get('danmaku-layer');
 const dmPositions=typeof document.querySelectorAll==='function'?[...document.querySelectorAll('.dm-positions button')]:[],dmSizes=typeof document.querySelectorAll==='function'?[...document.querySelectorAll('.dm-sizes button')]:[],dmSpeeds=typeof document.querySelectorAll==='function'?[...document.querySelectorAll('.dm-speeds button')]:[];
 const dmOpacity=get('dm-opacity'),dmOpacityValue=get('dm-opacity-value'),dmEnabled=get('dm-enabled');
 function applyDanmaku(){
   if(!dmLayer)return;
   dmLayer.hidden=!config.danmakuAvailable||!config.danmakuEnabled;
    if(dmButton)dmButton.hidden=!config.danmakuAvailable;
    if(!config.danmakuAvailable&&dmPanel)dmPanel.hidden=true;
   if(dmLayer.dataset){dmLayer.dataset.pos=config.danmakuPosition;dmLayer.dataset.size=config.danmakuSize;dmLayer.dataset.speed=config.danmakuSpeed;}
   if(dmLayer.style)dmLayer.style.opacity=String(Math.min(100,Math.max(10,config.danmakuOpacity))/100);
   if(dmPositions.length)for(const b of dmPositions)b.setAttribute('aria-pressed',String(b.dataset.pos===config.danmakuPosition));
   if(dmSizes.length)for(const b of dmSizes)b.setAttribute('aria-pressed',String(b.dataset.size===config.danmakuSize));
   if(dmSpeeds.length)for(const b of dmSpeeds)b.setAttribute('aria-pressed',String(b.dataset.speed===config.danmakuSpeed));
   if(dmOpacity){dmOpacity.value=String(config.danmakuOpacity);dmOpacityValue.textContent=config.danmakuOpacity+'%';dmOpacity.style.setProperty('--dm-progress',config.danmakuOpacity+'%');}
   if(dmEnabled)dmEnabled.checked=!!config.danmakuEnabled;
   if(dmButton)dmButton.setAttribute('aria-pressed',String(!!config.danmakuEnabled));
 }
 if(dmPanel&&dmButton){
 function closeDm(){dmPanel.hidden=true;dmButton.setAttribute('aria-expanded','false');reveal();}
 function toggleDmPanel(forceOpen){if(!config.danmakuAvailable)return;closeMenu();const open=forceOpen===true?true:dmPanel.hidden;dmPanel.hidden=!open;dmButton.setAttribute('aria-expanded',String(open));reveal();}
 for(const b of dmPositions)b.addEventListener('click',()=>{config.danmakuPosition=b.dataset.pos;applyDanmaku();void action('danmakuPosition',b.dataset.pos);});
 for(const b of dmSizes)b.addEventListener('click',()=>{config.danmakuSize=b.dataset.size;applyDanmaku();void action('danmakuSize',b.dataset.size);});
 for(const b of dmSpeeds)b.addEventListener('click',()=>{config.danmakuSpeed=b.dataset.speed;applyDanmaku();void action('danmakuSpeed',b.dataset.speed);});
 if(dmOpacity)dmOpacity.addEventListener('input',()=>{config.danmakuOpacity=Number(dmOpacity.value);applyDanmaku();void action('danmakuOpacity',dmOpacity.value);});
 if(dmEnabled)dmEnabled.addEventListener('change',()=>{config.danmakuEnabled=dmEnabled.checked;applyDanmaku();void action('danmaku',dmEnabled.checked?'true':'false');});
 dmButton.addEventListener('mouseenter',()=>{cancelMenuClose();toggleDmPanel(true);anchor(dmPanel,dmButton);});
 dmButton.addEventListener('mouseleave',()=>{setTimeout(()=>{if(dmPanel.hidden)return;if(dmPanelHovered)return;dmPanel.hidden=true;dmButton.setAttribute('aria-expanded','false');reveal();},180);});
 dmPanel.addEventListener('mouseenter',()=>{dmPanelHovered=true;});
 dmPanel.addEventListener('mouseleave',()=>{dmPanelHovered=false;dmPanel.hidden=true;dmButton.setAttribute('aria-expanded','false');reveal();});
 dmButton.addEventListener('click',()=>{toggleDmPanel(true);anchor(dmPanel,dmButton);});
 }
 // 点面板外收起，但不要吃掉视频的单击手势。
 if(typeof document.addEventListener==='function'&&dmPanel&&dmButton)document.addEventListener('pointerdown',event=>{if(dmPanel.hidden)return;if(dmPanel.contains(event.target)||dmButton.contains(event.target))return;closeDm();},true);
 applyDanmaku();
 renderConfig=()=>{get('player-title').textContent=config.title;get('episode-label').textContent=config.episode>0?'第 '+config.episode+' 集':'';get('quality').textContent=config.quality||'画质';get('previous').disabled=!config.canPrevious;get('next').disabled=!config.canNext;get('fullscreen').setAttribute('aria-label',config.fullscreen?'退出全屏':'全屏');icon('fullscreen',config.fullscreen?'exit':'fullscreen');get('window-fullscreen').disabled=config.fullscreen;get('window-fullscreen').setAttribute('aria-label',config.windowFullscreen?'退出窗口全屏':'窗口全屏');get('window-fullscreen').setAttribute('title',config.windowFullscreen?'退出窗口全屏（Esc）':'窗口全屏');icon('window-fullscreen',config.windowFullscreen?'windowExit':'window');applyDanmaku();if(opened)drawMenu();reveal();};
 icon('previous','previous');icon('next','next');icon('settings','settings');
 get('toggle-play').addEventListener('click',toggle);video.addEventListener('click',()=>{closeMenu();toggle();});video.addEventListener('dblclick',()=>full(!config.fullscreen));
 get('previous').addEventListener('click',()=>{if(config.canPrevious)void action('previous');});get('next').addEventListener('click',()=>{if(config.canNext)void action('next');});get('fullscreen').addEventListener('click',()=>full(!config.fullscreen));
 get('window-fullscreen').addEventListener('click',()=>{if(!config.fullscreen)void action('windowFullscreen',!config.windowFullscreen);});
 icon('pip','pip');
 get('pip').addEventListener('click',()=>{
   // 页面内能直接进画中画就自己处理；否则交给应用侧（MyGo 的 Page.TogglePictureInPicture）。
   if(typeof document.exitPictureInPicture==='function'&&document.pictureInPictureElement){void document.exitPictureInPicture();reveal();return;}
   if(typeof video.requestPictureInPicture==='function'){
     video.requestPictureInPicture().then(()=>reveal()).catch(()=>{void action('pip');});
     return;
   }
   void action('pip');
 });
 function seekTo(){if(!Number.isFinite(video.duration)||video.duration<=0)return;const position=Number(seek.value)/1000*video.duration;window.playerCommand('seek',position);seek.style.setProperty('--progress',Number(seek.value)/10+'%');preview.textContent=clock(position);preview.hidden=false;reveal();}
 seek.addEventListener('pointerdown',()=>{dragging=true;reveal();});seek.addEventListener('input',seekTo);seek.addEventListener('change',()=>{dragging=false;preview.hidden=true;sync();reveal();});window.addEventListener('pointerup',()=>{if(dragging){dragging=false;preview.hidden=true;sync();reveal();}});window.addEventListener('pointercancel',()=>{dragging=false;preview.hidden=true;reveal();});
 document.addEventListener('pointermove',reveal);document.addEventListener('pointerdown',event=>{if(!menu.contains(event.target)&&!menuButtons.some(id=>get(id).contains(event.target)))closeMenu();reveal();});ui.addEventListener('focusin',reveal);ui.addEventListener('focusout',()=>setTimeout(reveal,0));
 document.addEventListener('keydown',event=>{const target=event.target,tag=target&&target.tagName;if(event.key==='Escape'){event.preventDefault();closeMenu();if(config.fullscreen)full(false);else if(config.windowFullscreen)void action('windowFullscreen',false);return;}if(event.altKey||event.ctrlKey||event.metaKey||tag==='INPUT'||tag==='TEXTAREA'||tag==='SELECT'||(target&&target.isContentEditable))return;if((event.code==='Space'||event.key===' ')&&tag==='BUTTON')return;switch(event.key){case ' ':case 'Spacebar':event.preventDefault();toggle();break;case 'ArrowLeft':case 'ArrowRight':event.preventDefault();window.playerCommand('seek',Math.max(0,Math.min(Number.isFinite(video.duration)?video.duration:Infinity,video.currentTime+(event.key==='ArrowLeft'?-5:5))));sync();break;case 'f':case 'F':event.preventDefault();full(!config.fullscreen);break;default:return;}reveal();});
 for(const event of ['timeupdate','loadedmetadata','play','pause','ended','ratechange','volumechange','seeked'])video.addEventListener(event,()=>{sync();if(event!=='timeupdate')reveal();});
 window.addEventListener('pagehide',()=>clearTimeout(idleTimer));renderConfig();sync();reveal();
}
// 弹幕层：按视频时间轴投放，按轨道调度避免重叠；排不下的直接丢弃。
var danmakuCues=[],danmakuNext=0,danmakuActive=new Map(),danmakuLanes=[],danmakuOn=true,danmakuLayer=document.getElementById('danmaku-layer');
// 单条弹幕从右飘到左的秒数：默认适中，比原来的 7 秒慢一些。
const DANMAKU_LIVES={slow:14,medium:10,fast:7};
function danmakuLife(){const speed=(danmakuLayer&&danmakuLayer.dataset&&danmakuLayer.dataset.speed)||'medium';return DANMAKU_LIVES[speed]||DANMAKU_LIVES.medium;}
window.playerDanmaku=cues=>{for(const n of danmakuActive.values())n.remove();danmakuActive.clear();danmakuLanes.length=0;danmakuNext=0;danmakuCues=Array.isArray(cues)?cues.filter(c=>c&&Number.isFinite(c.time)&&typeof c.text==='string'&&c.text.trim()).map(c=>({time:c.time,text:c.text})).sort((a,b)=>a.time-b.time):[];return danmakuCues.length;};
window.playerDanmakuEnabled=on=>{danmakuOn=!!on;if(danmakuLayer&&danmakuLayer.classList)danmakuLayer.classList.toggle('danmaku-off',!danmakuOn);return danmakuOn;};
if(danmakuLayer&&typeof danmakuLayer.appendChild==='function'&&typeof requestAnimationFrame==='function'){
  // 不同位置可用轨道数不同；轨道排满时丢弃新弹幕，避免糊成一片。
  function laneCount(){const pos=(danmakuLayer.dataset&&danmakuLayer.dataset.pos)||'full';return pos==='mega'?10:pos==='top'?3:pos==='half'?4:8;}
  // 轨道间隔随速度和字号变化，避免慢速时挤在一起。
  function laneGap(){const size=(danmakuLayer.dataset&&danmakuLayer.dataset.size)||'medium';const ratio=size==='large'?.22:size==='small'?.15:.18;return danmakuLife()*ratio;}
  function lowerBound(t){let lo=0,hi=danmakuCues.length;while(lo<hi){const mid=(lo+hi)>>1;if(danmakuCues[mid].time<t)lo=mid+1;else hi=mid;}return lo;}
  function clearAll(){for(const n of danmakuActive.values())n.remove();danmakuActive.clear();danmakuLanes.length=0;}
  function place(i){
    const cue=danmakuCues[i],n=laneCount();
    if(danmakuLanes.length!==n)danmakuLanes.length=n;
    let lane=-1;
    for(let k=0;k<n;k++)if((danmakuLanes[k]||0)<=cue.time){lane=k;break;}
    if(lane<0)return;
    danmakuLanes[lane]=cue.time+laneGap();
    const node=document.createElement('div');
    node.className='danmaku-item';
    node.textContent=cue.text;
    node.style.top=(lane*(100/n)+2)+'%';
    danmakuLayer.appendChild(node);
    danmakuActive.set(i,node);
  }
  function tick(){
    requestAnimationFrame(tick);
    const now=video.currentTime;
    if(!danmakuOn||!Number.isFinite(now))return;
    if(danmakuNext>0&&danmakuCues[danmakuNext-1]&&danmakuCues[danmakuNext-1].time>now+0.5){clearAll();danmakuNext=lowerBound(now);}
    while(danmakuNext<danmakuCues.length&&danmakuCues[danmakuNext].time<=now){place(danmakuNext);danmakuNext++;}
    const width=danmakuLayer.clientWidth||1;
    for(const [i,node] of danmakuActive){
      const life=danmakuLife();
      const elapsed=now-danmakuCues[i].time;
      if(elapsed>life){node.remove();danmakuActive.delete(i);continue;}
      if(elapsed<0)continue;
      const travel=width+(node.offsetWidth||120);
      node.style.transform='translateX('+(width-elapsed/life*travel).toFixed(1)+'px)';
    }
  }
  requestAnimationFrame(tick);
  video.addEventListener('seeked',()=>{clearAll();danmakuNext=lowerBound(video.currentTime);});
}
