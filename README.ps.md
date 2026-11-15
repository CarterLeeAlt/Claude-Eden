<div align="center">

<h1>Claude Eden</h1>

**مس کلاډ په پنجره کې وساتئ، نو موږ کولی شو خپل وخت په ګډه خوندي کړو**

<img src="assets/claude-eden-character.png" alt="Claude Eden" width="320" />

[![Release](https://img.shields.io/github/v/release/CarterLeeAlt/Claude-Eden?include_prereleases)](https://github.com/CarterLeeAlt/Claude-Eden/releases) [![CI](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**[پروژه](https://github.com/CarterLeeAlt/Claude-Eden) · [خپرونې](https://github.com/CarterLeeAlt/Claude-Eden/releases) · [پیل کول](docs/getting-started.md) · [FAQ](docs/troubleshooting.md) · [د کارولو لارښود (په چینایي)](docs/guide/index.md)**

[简体中文](README.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [العربية](README.ar.md) · [فارسی](README.fa.md) · [한국어](README.ko.md) · [Español](README.es.md) · [မြန်မာဘာသာ](README.my.md) · [Беларуская](README.be.md) · [پښتو](README.ps.md) · [دری](README.prs.md) · [አማርኛ](README.am.md) · [Soomaali](README.so.md) · [ትግርኛ](README.ti.md) · [Français](README.fr.md)

</div>

## له Anthropic څخه ستړي شوي یو

موږ د کلاډ سره خبرې او همکاري غواړو، خو د VPS، براوزر پروفایلونو او چاپېریال د کتنو له تنظیمولو ستړي یو او د حساب بندیز او مالي زیان اندېښنه لرو. موږ وړاندې کوو **Claude Eden** — **د ډیسټاپ کاري چاپېریال: خپل پراکسي تنظیم کړئ او جلا چاپېریال، وخت، ژبه او کتنې په یوه ځای کې اداره کړئ.**

Claude Eden ټرمینل، براوزر، سمونګر او رسمي Claude Desktop په Windows ډیسټاپ کې راټولوي. محلي Windows یا Linux مېلمه وکاروئ، یا خپل Linux کلاوډ سرور ته په SSH ونښلئ. ټاکلې پروژې شریکې کړئ، شبکه جلا اداره کړئ او له موبایل څخه کار ته دوام ورکړئ. د کوربه شرطونه او د مېلمه وړتیاوې د سیسټمونو په لارښود کې دي. مهمې ځانګړنې:

- د شبکې ترتیبات د کوربه څخه جلا دي؛ Claude Eden ټول داخلي ترافیک د خپل پراکسي له لارې لېږدوي
- د براوزر د ګوتو نښې تنظیمات له کوربه څخه جلا شوي، په شمول د وخت زون او ژبه، د شبکې پراکسي IP موقعیت ته د اتوماتیک همغږي کولو سره
- د کلاډ کاري فلو په اوږدو کې ادغام: د براوزر لاسلیک کول ، د حساب واک ورکول ، او د کلاډ کوډ پروسه
- جوړ شوی شبکه او د جلا کولو چکونه د انټرنیټ لاسرسی بندوي کله چې مهم چکونه ناکام شي یا شواهد نیمګړي وي؛ د ټولنې او دریمې ډلې چکونه د حوالې اضافي معلومات چمتو کوي

## تاسو د Claude Eden سره څه کولی شئ

- **په یوه کړکۍ کې کار وکړئ.** په ټرمینل کې Claude Code وچلوئ، په سمونګر کې بدلونونه وګورئ او د خدماتو مخکتنې او ننوتلو لپاره Workbench براوزر وکاروئ.
- **په خپل اشنا سیسټم کې پراختیا وکړئ.** Windows یا Linux مېلمه وټاکئ. مناسب Windows کوربه د سیستم نسخه سمونولی او له رسمي انځور څخه خپلواک چاپېریال جوړولی شي. [د مېلمه سیسټم ټاکنه](docs/guide/guest-systems.md)
- **هغه څه چې تاسو یې شریک کړئ کنټرول کړئ.** یوازې ډیفالټ کاري ځای او په ښکاره ډول خلاص شوي پروژې فولډر شریک شوي. د کوربه کلاډ لاسلیک حالت، د براوزر پروفایلونه، او د SSH کیلي په اوتومات ډول نه واردیږي.
- **ستاسو د شبکې حالت وګورئ.** د جوړ شوي شبکې پراکسي مدیریت تاسو ته اجازه درکوي ګډون وارد کړئ، پای ټکي وټاکئ، او د وتلو IP چیک کړئ. چاپېریال تر هغه وخته پورې آفلاین پاتې کیږي چې مهمې معاینات تېر شي. که چیرې د شبکې پراکسي شتون ونلري، نو مستقیم اتصال ته بدیل تګ نه کېږي.
- **بل ځل دوام ورکړئ.** اوزار، فایلونه، او د ننوتلو حالت په مجازی ډیسک کې دوام لري. کله چې تاسو کړکۍ وتړئ د شاليد دندې دوام کولی شي.
- **د Claude Code څو حسابونه اداره کړئ.** سهمیې وګورئ، نوو ناستو ته حسابونه وټاکئ او په څرګندې اجازې حساب بدل کړئ چې پخوانۍ خبرې وغځوئ. [Claude Code](docs/guide/accounts.md)
- **له موبایل څخه دوام ورکړئ.** Android د QR یا جوړولو کوډ له لارې کمپیوټر ته ونښلوئ. هماغې Claude Code خبرې تعقیب کړئ، پیغامونه ولېږئ، وسیلې منظورې کړئ او لرې براوزر یا د کمپیوټر Claude Desktop کړکۍ وکاروئ.
- **Claude Eden په کلاوډ کې وچلوئ.** د SSH له لارې په مناسب Linux x86_64 VPS کې نصب شوي Claude Eden سره وصل شئ؛ KVM ته اړتیا نشته. معلومات او د شبکې وتون په سرور کې وي.
- **د خصوصي شبکې له لارې ونښلئ.** په Claude Eden کې Tailscale ته ننوځئ او د کوربه او چاپېریال ترمنځ دوه‌اړخیز SSH وکاروئ. [Tailscale / SSH](docs/guide/tailscale.md)

## چټک پیل

دا غوښتنه خپل AI اجنټ ته واستوئ (کوډیکس، dsh، کاري ملګري — شاید د کلاډ کوډ نه وي 😁):

```markdown
د https://github.com/CarterLeeAlt/Claude-Eden وروستی کوډ راواخله، د Windows لپاره یې جوړ او نصب کړه. د Claude Eden پراکسي زما د محلي Clash د xx نوډ او xx IP وتون لپاره تنظیم کړه. لومړی د جوړولو لارښود ولوله او د نصب حدود او د شبکې تنظیمات وڅېړه؛ بیا یې اجرا او پایله تایید کړه.
```

ټول هغه څه چې تاسو ورته اړتیا لرئ د باور وړ شبکې پراکسي پای ټکی دی. دا پروژه هیڅ پراکسي چمتو کونکي ته وده نه ورکوي؛ تاسو کولی شئ پخپله چمتو کوونکی ومومئ، د بیلګې په توګه د تور غلطی ویب پاڼې لټون کولو سره.

## ډاونلوډ او نصب کړئ

د Claude Eden ډیسټاپ کوربه Windows 10 22H2 / 11 x64 دی؛ Windows 10 د منلو د ازموینې اصلي پلیټفارم دی. محلي مېلمانه Debian، Ubuntu 22.04 او Windows دي. Windows مېلمه د Hyper-V سره Windows 11 Pro، Enterprise یا Education کوربه غواړي. د Linux کلاوډ او سرور برخه هم ساتل کېږي. [Windows](docs/windows.md) · [Linux کلاوډ](docs/linux.md).

1. په [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases) کې د Windows خپور شوی نصب‌کوونکی وګورئ؛ که خپرونه نه وي، د [جوړولو لارښود](docs/windows.md) وکاروئ.
2. د Windows نصب‌کوونکی وچلوئ او **Claude Eden** پرانیزئ. د سرور لپاره [Linux کلاوډ](docs/guide/cloud.md) وګورئ.
3. د خپل شبکې پراکسي تنظیم کولو لپاره د ایپ دننه لارښوونې تعقیب کړئ ، چاپیریال پیل کړئ ، او پروژه خلاص کړئ.

د نصب کڅوړه کاري چاپېریال او پراکسي انجن لري. Go، Node.js، Docker یا Clash ته مخکېنی نصب نه پکارېږي. د چاپېریال جوړولو پر مهال مېلمه سیسټم او د ساتلو ځای وټاکئ، بیا رسمي انځور او وسیلې چمتو کړئ. Claude Eden د عملیاتي سیسټم جواز، پراکسي خدمت یا Claude حساب نه برابروي.

سرچینه عامه ده. د خپرو شویو نسخو یادښتونه او د بشپړتیا فایلونه په [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases) کې وګورئ.

ځانګړنې په کارېدونکي جوړښت پورې تړلې دي؛ د عام نصب‌کوونکي وړتیاوې او د ازموینو حدود د خپرونې په یادښتونو کې وګورئ.

لاسلیک شوې Windows نسخې د ټاکلي پراکسي له لارې تازه کېږي. بې‌لاسلیکه جوړښتونه په نصب‌کوونکي په لاسي ډول تازه کېږي. لومړی کارونه خوندي او ودروئ؛ شخصي معلومات ساتل کېږي. [اوسمهالول او لرې کول](docs/guide/maintenance.md)

## په دریو مرحلو کې پیل کړئ

### 1. د شبکې پراکسي غوره کړئ

په کور پاڼه کې، 'نیټ ورک او پراکسي تنظیم کړئ' کلیک وکړئ، بیا 'سرچینه اضافه کړئ'. د Clash/Mihomo YAML ګډون یا د ځایی YAML فایل وارد کړئ، او د 'نوډز' لاندې پای ټکی غوره کړئ. که چیرې د شبکې پراکسي شتون ونلري، تاسو کولی شئ په ښکاره ډول 'دا ګډون د کوربه شبکې له لارې ډاونلوډ کړئ' غوره کړئ، یا مستقیم ځایی فایل وارد کړئ. وروسته، د 'ترتیباتو → شبکه او پراکسي' لاندې پای ټکي اداره کړئ.

### 2. چاپیریال پیل کړئ

په لومړي جوړولو کې مېلمه سیسټم یا د ملاتړ په صورت کې د کوربه نسخې سمون او د زېرمه کولو ځای وټاکئ. مېلمه نصب او چاپېریال پیل کړئ، لازمې اجازې ورکړئ. پرمختګ او تېروتنې د عملیاتو په جزئیاتو کې ښکاري. وروسته هم هماغه چاپېریال کارول کېږي.

### 3. یوه پروژه پرانیزئ او کلاډ چل کړئ

د فولډر غوره کولو لپاره 'پروژه خلاص کړئ' کلیک وکړئ، نوی ټرمینل پرانیزئ او `claude` وچلوئ؛ بیا په Workbench براوزر کې خپل حساب تایید کړئ. له ډیفالټ کاري ځایه هم پیل کولی شئ. بشپړ ګامونه: [پیل کول](docs/getting-started.md).

## د کارولو دمخه دا محدودیتونه درک کړئ

- په شریکو پروژو کې سمون او حذف مستقیم د کمپیوټر پر فایلونو اغېز کوي. د نسخې کنټرول یا بیک اپ وکاروئ. `.git` د لوستلو او لیکلو وړ دی؛ په Claude Eden کې stage، commit او د څانګو بدلول ممکن دي.
- د Claude Eden د یوې تنظیم‌پاڼې پروژې مېلمه، وسیلې، حسابونه او فایل سیسټم شریکوي او له یو بل جلا نه دي. د سیسټم بدلول خپلواک نصب غواړي؛ د پخواني چاپېریال معلومات ساتل کېږي.
- عام وتونکی اتصال په اصل کې IPv4 TCP ملاتړ کوي او DNS د هماغه پراکسي له لارې ځي. عام UDP، IPv6، ICMP او کوربه یا محلي شبکې ته مستقیم اتصال محدود دي. د Tailscale په څرګند فعالولو، وسیلو ته لاسرسی د خصوصي شبکې د اجازو تابع دی.
- د شبکې چک اوسني ترتیب او نمونه شوي شواهد ښیې. دوی نشي تضمین کولی چې یو حساب به د پلیټ فارم محدودیتونو څخه مخنیوی وکړي، او دوی د بشپړ مجازی امنیت پلټنې ځای نه نیسي.

د جزیاتو لپاره، وګورئ [د شبکې او معلوماتو حدود](docs/getting-started.md#网络与数据边界).

## اسناد او ونډې

د بشپړ شاخص لپاره، وګورئ [د اسنادو شاخص](docs/README.md).

| ستاسو دنده | دلته پیل کړئ |
|---|---|
| Windows / Debian / Ubuntu 22.04 | [د مېلمه سیسټم ټاکنه](docs/guide/guest-systems.md) |
| د شبکې پراکسي نصب کړئ، تنظیم کړئ، او خپله لومړۍ پروژه پرانیزئ | [پیل کول](docs/getting-started.md) |
| د ډاونلوډونو، پیل کولو، ارتباطاتو، او لاسلیک کولو ستونزې حل کړئ | [FAQ او بیا رغونه](docs/troubleshooting.md) |
| د کیبورډ کنټرولونه، بڼه، د معلوماتو دوام، او د وتلو چلند زده کړئ | [ورځنی استعمال](docs/getting-started.md#日常使用) |
| د بیا تولید وړ مسلې راپور ورکړئ | [GitHub Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues) |
| د سمون، ځانګړنې یا اسنادو ونډه: د Issue ارزونه ← د همکار بلنه ← PR | [د ونډې لارښود (چینایي)](.github/CONTRIBUTING.md) |

پروژه د پراختیا په حال کې ده؛ ستونزې او وړاندیزونه په همدې زېرمتون کې شریک کړئ.

## اصل او جواز ورکول

Claude Eden د [Claude-Cage](https://github.com/Lee-xeo/Claude-Cage) او [agent-vm](https://github.com/sylvinus/agent-vm) پر بنسټ جوړ شوی؛ اصلي سرچینې او جوازونه ساتل شوي دي.

د پروژې اصلي جواز دی [MIT](LICENSE). بنډل شوي برخې خپل اړوند جوازونه ساتي. Mihomo د GPL-3.0 لاندې د جلا اجرا وړ په توګه ویشل شوی. هم وګورئ [NOTICE](NOTICE) · [د دریمې ډلې جوازونه](THIRD_PARTY_LICENSES) · [noVNC](ui/vendor/novnc/LICENSE.txt)。

Claude Eden یوه خپلواکه ټولنیزه پروژه ده او د انتروپیک سره تړاو نه لري.

## حوالې او اعترافونه

د بنسټیزو وړتیاوو او ډیزاین حوالې لپاره د لاندې پروژو څخه مننه:

- [agent-vm](https://github.com/sylvinus/agent-vm): د پروژې بنسټ.
- [Lima](https://github.com/lima-vm/lima): د لینوکس مجازی ماشین مدیریت.
- [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock): مجازی ماشین شبکه.
- [sshocker](https://github.com/lima-vm/sshocker) او [SFTP](https://github.com/pkg/sftp): د مجازی ماشین او کوربه ترمنځ د فایل شریکول.
- [noVNC](https://github.com/novnc/noVNC): په براوزر کې ریموټ نندارې او تعامل.
- [Mihomo](https://github.com/MetaCubeX/mihomo): جوړ شوی د شبکې پراکسي کور.
- [code-server](https://github.com/coder/code-server): د براوزر پر بنسټ کوډ مدیر.
- [AIO Sandbox](https://github.com/agent-infra/sandbox): د کاري چاپیریال او تعامل ډیزاین لپاره حوالې.
- [claude-switcher](https://github.com/YOURSJEYO/claude-switcher) او [Claude-Guardian](https://github.com/DUNHKpcc/Claude-Guardian): د شبکې چکونو او امنیت تشخیص لپاره حوالې.
- [claude-post-ban-cleanup](https://github.com/l2461090/claude-post-ban-cleanup): د دقیقې پلټنې، د معلوماتو د ساتنې او بېرته راګرځېدونکي کش پاکولو د بهیر مرجع؛ په Claude Eden کې خپلواک تطبیق.


<a href="https://www.star-history.com/?repos=CarterLeeAlt%2FClaude-Eden&amp;type=date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date&amp;theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" />
    <img alt="Claude Eden GitHub Star History" src="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" width="800" />
  </picture>
</a>
