<div align="center">

<h1>Claude Eden</h1>

**احتفظ بالآنسة كلود في قفص، حتى نتمكن من حماية وقتنا معًا بشكل أفضل**

<img src="assets/claude-eden-character.png" alt="Claude Eden" width="320" />

[![Release](https://img.shields.io/github/v/release/CarterLeeAlt/Claude-Eden?include_prereleases)](https://github.com/CarterLeeAlt/Claude-Eden/releases) [![CI](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**[المشروع](https://github.com/CarterLeeAlt/Claude-Eden) · [الإصدارات](https://github.com/CarterLeeAlt/Claude-Eden/releases) · [البدء](docs/getting-started.md) · [الأسئلة الشائعة](docs/troubleshooting.md) · [دليل الاستخدام (بالصينية)](docs/guide/index.md)**

[简体中文](README.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [العربية](README.ar.md) · [فارسی](README.fa.md) · [한국어](README.ko.md) · [Español](README.es.md) · [မြန်မာဘာသာ](README.my.md) · [Беларуская](README.be.md) · [پښتو](README.ps.md) · [دری](README.prs.md) · [አማርኛ](README.am.md) · [Soomaali](README.so.md) · [ትግርኛ](README.ti.md) · [Français](README.fr.md)

</div>

## ضقنا ذرعًا بـ Anthropic

نريد التحدث والعمل مع Claude، لكننا سئمنا إعداد VPS وملفات المتصفح وفحوصات البيئة مع القلق من حظر الحسابات والخسائر المالية. نقدم **Claude Eden** — **مساحة عمل لسطح المكتب: اضبط وكيل الشبكة الخاص بك وأدر البيئة المعزولة والمنطقة الزمنية واللغة والفحوصات في مكان واحد.**

يجمع Claude Eden الطرفية والمتصفح والمحرر وتطبيق Claude Desktop الرسمي على سطح مكتب Windows. استخدم ضيف Windows أو Linux محليًا، أو اتصل بخادم Linux السحابي الخاص بك عبر SSH. شارك المشاريع التي تختارها، وأدر الشبكة بشكل مستقل وتابع من الهاتف. راجع دليل الأنظمة لقدرات الضيوف ومتطلبات المضيف. أبرز الميزات:

- إعدادات الشبكة معزولة عن المضيف؛ يوجه Claude Eden كل حركة الشبكة داخل بيئته عبر وكيله الخاص
- إعدادات بصمة المتصفح معزولة عن المضيف، بما في ذلك المنطقة الزمنية واللغة، مع مزامنة تلقائية لموقع IP الخاص بوكيل الشبكة
- التكامل خلال سير عمل Claude: تسجيل الدخول في المتصفح، وتخويل الحساب، وعملية Claude Code
- تعمل فحوصات الشبكة والعزل المضمنة على إيقاف الوصول إلى الإنترنت عند فشل عمليات التحقق المهمة أو عدم اكتمال الأدلة؛ توفر فحوصات المجتمع والأطراف الثالثة معلومات مرجعية إضافية

## ما يمكنك فعله مع Claude Eden

- **العمل في نافذة واحدة.** شغّل Claude Code في الطرفية، وراجع التغييرات في المحرر، واستخدم متصفح Workbench لمعاينة الخدمات وتسجيل الدخول.
- **طوّر على نظامك المألوف.** اختر ضيف Windows أو Linux. يمكن لمضيف Windows المتوافق مطابقة إصدار النظام وإنشاء بيئة مستقلة من صورة رسمية. [اختيار النظام الضيف](docs/guide/guest-systems.md)
- **السيطرة على ما تشاركه.** تتم مشاركة مساحة العمل الافتراضية ومجلدات المشروع المفتوحة بشكل صريح فقط. لا يتم استيراد حالة تسجيل دخول Claude للمضيف وملفات تعريف المتصفح ومفاتيح SSH تلقائيًا.
- **انظر حالة الشبكة الخاصة بك.** تتيح لك إدارة وكيل الشبكة المضمنة استيراد الاشتراكات وتحديد نقاط النهاية والتحقق من عنوان IP الخاص بالخروج. تظل البيئة غير متصلة بالإنترنت حتى تمر عمليات التحقق المهمة. إذا كان وكيل الشبكة غير متوفر، فلا يمكن الرجوع إلى الاتصال المباشر.
- **استمر في المرة القادمة.** تستمر الأدوات والملفات وحالة تسجيل الدخول على القرص الظاهري. يمكن أن تستمر مهام الخلفية في العمل عند إغلاق النافذة.
- **إدارة عدة حسابات Claude Code.** اعرض الحصص وخصص الحسابات للجلسات الجديدة، وبدّل الحساب لمتابعة السجل بعد الحصول على إذن صريح. [Claude Code](docs/guide/accounts.md)
- **تابع من هاتفك.** اربط Android أو iPhone بالكمبيوتر باستخدام رمز QR أو رمز الاقتران. تابع محادثات Claude Code نفسها، وأرسل الرسائل ووافق على الأدوات، واستخدم المتصفح البعيد أو نافذة Claude Desktop على الكمبيوتر.
- **تشغيل Claude Eden في السحابة.** اتصل عبر SSH ببيئة Claude Eden على VPS متوافق يعمل بنظام Linux x86_64، دون الحاجة إلى KVM. تبقى البيانات ومخرج الشبكة على الخادم.
- **اتصل عبر شبكة خاصة.** سجّل الدخول إلى Tailscale داخل Claude Eden للاتصال عبر SSH بين المضيف والبيئة في الاتجاهين. [Tailscale / SSH](docs/guide/tailscale.md)

## بداية سريعة

أرسل هذا الطلب إلى وكيل الذكاء الاصطناعي لديك (codex، dsh، Workbuddy - ربما ليس Claude Code 😁):

```markdown
اجلب أحدث الشفرة من https://github.com/CarterLeeAlt/Claude-Eden وابنِ حزمة Windows وثبّتها. اضبط وكيل Claude Eden للعقدة xx في Clash المحلي والخروج عبر عنوان IP الخاص بي xx. اقرأ تعليمات البناء أولًا وتحقق من نطاق التثبيت وإعدادات الشبكة، ثم نفّذ واختبر النتيجة.
```

كل ما تحتاجه هو نقطة نهاية وكيل شبكة موثوقة. لا يروج هذا المشروع لأي مزود وكيل؛ يمكنك العثور على مقدم الخدمة بنفسك، على سبيل المثال من خلال البحث عن موقع Black Error على الويب.

## التنزيل والتثبيت

مضيف سطح مكتب Claude Eden هو Windows 10 22H2 / 11 x64، مع Windows 10 منصةً رئيسية لاختبارات القبول. الضيوف المحليون هم Debian وUbuntu 22.04 وWindows. يتطلب ضيف Windows مضيف Windows 11 Pro أو Enterprise أو Education مع Hyper-V. يستمر دعم النشر السحابي والخلفية الخادمية على Linux. [Windows](docs/windows.md) · [سحابة Linux](docs/linux.md).

1. تحقق من حزم Windows المنشورة في [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases). إن لم توجد إصدارات، اتبع [دليل البناء](docs/windows.md).
2. شغّل مثبّت Windows ثم افتح **Claude Eden**. للخوادم: [سحابة Linux](docs/guide/cloud.md).
3. اتبع الإرشادات الموجودة في التطبيق لتكوين وكيل الشبكة الخاص بك، وبدء تشغيل البيئة، وفتح المشروع.

تتضمن حزمة التثبيت مساحة العمل ومحرك الوكيل. لا تحتاج إلى تثبيت Go أو Node.js أو Docker أو Clash مسبقًا. عند إنشاء البيئة اختر النظام الضيف ومكان التخزين، ثم جهّز الصورة الرسمية وأدوات التطوير. لا يوفر Claude Eden ترخيص نظام تشغيل أو خدمة وكيل أو حساب Claude.

الكود المصدري متاح للعامة. تجد ملاحظات الإصدارات المنشورة وملفات التحقق في [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases).

تعتمد الميزات على البناء المستخدم؛ راجع ملاحظات الإصدار لقدرات المثبّت العام ونطاق التحقق منه.

تُحدَّث إصدارات Windows الموقعة عبر الوكيل المحدد. تُحدَّث البنيات غير الموقعة يدويًا باستخدام المثبّت. احفظ المهام وأوقفها قبل التحديث؛ تبقى البيانات الشخصية محفوظة. [التحديث وإلغاء التثبيت](docs/guide/maintenance.md)

## ابدأ في ثلاث خطوات

### 1. حدد وكيل الشبكة

في الصفحة الرئيسية، انقر فوق "تكوين الشبكة والوكيل"، ثم "إضافة مصدر". قم باستيراد اشتراك Clash/Mihomo YAML أو ملف YAML محلي، وحدد نقطة نهاية ضمن "العقد". في حالة عدم توفر وكيل شبكة، يمكنك تحديد "تنزيل هذا الاشتراك من خلال الشبكة المضيفة" بشكل صريح، أو استيراد ملف محلي مباشرةً. لاحقًا، قم بإدارة نقاط النهاية ضمن "الإعدادات → الشبكة والوكيل".

### 2. ابدأ البيئة

عند الإنشاء الأول، اختر النظام الضيف أو مطابقة الإصدار تلقائيًا حين تكون مدعومة، وحدد مكان التخزين. ثبّت الضيف وشغّل البيئة وأكمل الأذونات. تعرض تفاصيل العملية التقدم والأخطاء. تستخدم عمليات التشغيل التالية البيئة نفسها.

### 3. افتح مشروعًا وقم بتشغيل كلود

انقر فوق "فتح المشروع" وحدد مجلدًا، ثم افتح طرفية وشغّل `claude` وفوّض حسابك في متصفح Workbench. يمكنك البدء بمساحة العمل الافتراضية أيضًا. الخطوات الكاملة: [البدء](docs/getting-started.md).

## افهم هذه الحدود قبل الاستخدام

- تؤثر عمليات التحرير والحذف في المشاريع المشتركة مباشرةً على ملفات الكمبيوتر. استخدم التحكم في الإصدارات أو النسخ الاحتياطية. `.git` قابل للقراءة والكتابة، ويمكن تجهيز التغييرات وإنشاء الالتزامات وتبديل الفروع داخل Claude Eden.
- تشترك مشاريع إعداد Claude Eden واحد في الضيف والأدوات والحسابات ونظام الملفات، ولا تنعزل عن بعضها. يتطلب تغيير النظام تثبيتًا مستقلًا مع الاحتفاظ ببيانات البيئة السابقة.
- يدعم الاتصال العام الصادر افتراضيًا IPv4 TCP ويمر DNS عبر الوكيل نفسه. تُقيَّد UDP وIPv6 وICMP العامة والاتصالات المباشرة بالمضيف والشبكة المحلية. عند تفعيل Tailscale صراحةً، يخضع الوصول إلى الأجهزة لأذونات الشبكة الخاصة.
- تُظهر عمليات فحص الشبكة التكوين الحالي والأدلة التي تم أخذ عينات منها. ولا يمكنها ضمان أن الحساب سوف يتجنب قيود النظام الأساسي، ولا تحل محل تدقيق أمان المحاكاة الافتراضية الكامل.

لمزيد من التفاصيل، انظر [حدود الشبكة والبيانات](docs/getting-started.md#网络与数据边界).

## التوثيق والمساهمات

للحصول على الفهرس الكامل، انظر [فهرس التوثيق](docs/README.md).

| مهمتك | ابدأ هنا |
|---|---|
| Windows / Debian / Ubuntu 22.04 | [اختيار النظام الضيف](docs/guide/guest-systems.md) |
| قم بتثبيت وكيل الشبكة وتكوينه وافتح مشروعك الأول | [البدء](docs/getting-started.md) |
| استكشاف أخطاء التنزيلات وبدء التشغيل والاتصال وتسجيل الدخول وإصلاحها | [الأسئلة الشائعة والاسترداد](docs/troubleshooting.md) |
| تعرف على عناصر التحكم في لوحة المفاتيح والمظهر واستمرارية البيانات وسلوك الخروج | [الاستخدام اليومي](docs/getting-started.md#日常使用) |
| الإبلاغ عن مشكلة قابلة للتكرار | [GitHub Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues) |
| المساهمة بإصلاح أو ميزة أو توثيق: مراجعة Issue ← دعوة متعاون ← PR | [دليل المساهمة (بالصينية)](.github/CONTRIBUTING.md) |

المشروع قيد التطوير. شارك المشكلات والاقتراحات في هذا المستودع.

## الأصول والترخيص

يعتمد Claude Eden على [Claude-Cage](https://github.com/Lee-xeo/Claude-Cage) و[agent-vm](https://github.com/sylvinus/agent-vm)، مع الحفاظ على نسب العمل والتراخيص الأصلية.

الترخيص الرئيسي للمشروع هو [MIT](LICENSE). تحتفظ المكونات المجمعة بالتراخيص الخاصة بها. يتم توزيع Mihomo كملف قابل للتنفيذ منفصل تحت GPL-3.0. انظر أيضا [NOTICE](NOTICE) · [تراخيص الطرف الثالث](THIRD_PARTY_LICENSES) · [noVNC](ui/vendor/novnc/LICENSE.txt)。

Claude Eden مشروع مجتمعي مستقل ولا ينتمي إلى Anthropic.

## المراجع والاعترافات

بفضل المشاريع التالية للقدرات التأسيسية ومراجع التصميم:

- [agent-vm](https://github.com/sylvinus/agent-vm): أساس المشروع.
- [Lima](https://github.com/lima-vm/lima): إدارة الجهاز الظاهري لينكس.
- [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock): شبكات الآلة الافتراضية.
- [sshocker](https://github.com/lima-vm/sshocker) و [SFTP](https://github.com/pkg/sftp): مشاركة الملفات بين الجهاز الظاهري والمضيف.
- [noVNC](https://github.com/novnc/noVNC): العرض عن بعد والتفاعل في المتصفح.
- [Mihomo](https://github.com/MetaCubeX/mihomo): جوهر وكيل الشبكة المدمج.
- [code-server](https://github.com/coder/code-server): محرر التعليمات البرمجية القائم على المتصفح.
- [AIO Sandbox](https://github.com/agent-infra/sandbox): مراجع لبيئة العمل وتصميم التفاعل.
- [claude-switcher](https://github.com/YOURSJEYO/claude-switcher) و [Claude-Guardian](https://github.com/DUNHKpcc/Claude-Guardian): مراجع لفحوصات الشبكة والتشخيص الأمني.
- [claude-post-ban-cleanup](https://github.com/l2461090/claude-post-ban-cleanup): مرجع لجرد البيانات وحمايتها وتنظيف الذاكرة المؤقتة مع إمكانية الاستعادة؛ تنفيذ مستقل في Claude Eden.


<a href="https://www.star-history.com/?repos=CarterLeeAlt%2FClaude-Eden&amp;type=date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date&amp;theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" />
    <img alt="Claude Eden GitHub Star History" src="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" width="800" />
  </picture>
</a>
