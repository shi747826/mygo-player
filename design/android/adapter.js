/* Load a real window.MyGoMobileAdapter before this script to replace demo data.
   The optional API contract and video/session lifecycle are described in DESIGN.md. */
'use strict';
window.MyGoMobileDemo = {
  sources: ['星河片库','微光剧场'],
  shows: [
    {title:'雾港来信',total:96,tag:'悬疑',desc:'一封迟到七年的信，让陌生的港口再次亮灯。',line:'「这一次，换我等你回来。」',heat:'12.8万'},
    {title:'晚风有回声',total:72,tag:'都市',desc:'在城市的另一端，听见彼此的答案。',line:'「风经过的地方，我都记得。」',heat:'8.6万'},
    {title:'第七次日出',total:120,tag:'治愈',desc:'与昨天告别，和明天重新相遇。',line:'「天亮以前，我们还有时间。」',heat:'6.2万'},
    {title:'月光便利店',total:48,tag:'奇幻',desc:'午夜零点开门，只收留未说出口的愿望。',line:'「你的愿望，今天到货了。」',heat:'9.1万'},
    {title:'春日慢半拍',total:36,tag:'治愈',desc:'慢一点，也能赶上属于自己的春天。',line:'「别着急，花还没开完。」',heat:'4.7万'},
    {title:'沿海公路的最后一站',total:84,tag:'都市',desc:'一路向海，找回被遗忘的自己。',line:'「下一站，就是我们。」',heat:'7.3万'},
    {title:'时间裁缝',total:60,tag:'奇幻',desc:'修补一段时光，也修补一颗心。',line:'「遗憾，也有它的纹路。」',heat:'3.9万'},
    {title:'等一场晴天',total:90,tag:'都市',desc:'雨停之后，故事才刚刚开始。',line:'「晴天总会来的。」',heat:'5.2万'}
  ].map((s,i)=>({...s,id:String(i),poster:`assets/poster-${i}.svg`,scene:`assets/scene-${i}.svg`}))
};
const mygoAdapter = window.MyGoMobileAdapter;
window.MyGoMobileBridge = {
  isDemo: !mygoAdapter,
  async call(method, payload) {
    if (!mygoAdapter) return null;
    if (typeof mygoAdapter[method] !== 'function') return null;
    return await mygoAdapter[method](payload);
  }
};
