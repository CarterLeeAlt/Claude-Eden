<div align="center">

<h1>Claude Eden</h1>

**Ku hay Miss Claude qafis, si aan si wanaagsan u ilaalin karno waqtigeena wada jirka ah**

<img src="assets/claude-eden-character.png" alt="Claude Eden" width="320" />

[![Release](https://img.shields.io/github/v/release/CarterLeeAlt/Claude-Eden?include_prereleases)](https://github.com/CarterLeeAlt/Claude-Eden/releases) [![CI](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**[Mashruuca](https://github.com/CarterLeeAlt/Claude-Eden) · [Sii-deynta](https://github.com/CarterLeeAlt/Claude-Eden/releases) · [Bilaabidda](docs/getting-started.md) · [FAQ](docs/troubleshooting.md) · [Hagaha isticmaalka (Shiine)](docs/guide/index.md)**

[简体中文](README.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [العربية](README.ar.md) · [فارسی](README.fa.md) · [한국어](README.ko.md) · [Español](README.es.md) · [မြန်မာဘာသာ](README.my.md) · [Беларуская](README.be.md) · [پښتو](README.ps.md) · [دری](README.prs.md) · [አማርኛ](README.am.md) · [Soomaali](README.so.md) · [ትግርኛ](README.ti.md) · [Français](README.fr.md)

</div>

## Waan ka daalnay Anthropic

Isticmaalayaal raba inay la shaqeeyaan Claude ahaan, waanu ka daalnay habaynta VPS, astaamaha biraawsarka iyo hubinta deegaanka, anagoo ka walwalsan xayiraadaha akoonnada iyo khasaaraha maaliyadeed. Waxaan soo bandhigaynaa **Claude Eden** — **goob shaqo oo desktop ah: habayso proxy-gaaga oo hal meel ku maamul deegaanka go'doonsan, aagga wakhtiga, luqadda iyo hubinnada.**

Claude Eden wuxuu terminalka, biraawsarka, tafatiraha iyo Claude Desktop-ka rasmiga ah ku mideeyaa Windows 10 oo leh marti Debian 13 oo maxalli ah. Wadaag mashaariicda aad doorato oo shabakadda si gaar ah u maamul. Awoodaha iyo kheyraadka martigeliyaha waxaa lagu hubiyaa kombiyuutar kasta. Astaamaha ugu waaweyn:

- Dejinta shabakadda way ka go'an tahay martigeliyaha; Claude Eden wuxuu dhammaan taraafikada gudaha mariyaa proxy-giisa
- Dejinta faraha browserka ee ka go'doon martigeliyaha, oo ay ku jiraan aaga wakhtiga iyo luqadda, oo leh si toos ah ula mid ah goobta wakiilka shabakada
- Isku dhafka inta lagu jiro socodka shaqada Claude: soo gelida browserka, oggolaanshaha akoontiga, iyo nidaamka Claude Code
- Shabakad la dhisay iyo jeegaga go'doominta ayaa hakiya gelitaanka intarneedka marka hubinta muhiimka ahi ay fashilmaan ama caddayntu aanay dhammaystirnayn; jeegaga bulshada iyo qolo saddexaad waxay bixiyaan macluumaad tixraac dheeraad ah

## Maxaad ku samayn kartaa Claude Eden

- **Ka shaqee hal daaqad.** Orod Claude Code gudaha terminalka, dib u eeg isbeddelada tifaftiraha, oo isticmaal biraawsarka Workbench si aad u aragto adeegyada oo aad u soo gasho.
- **Ku horumari deegaan Linux oo madax bannaan.** Isticmaal qalabka Linux gudaha Debian 13 oo maxalli ah, oo wadaag keliya mashaariicda aad si cad u oggolaatay. [Martida Debian 13](docs/guide/guest-systems.md)
- **Xakamee waxa aad wadaagto.** Kaliya goobta shaqada ee caadiga ah iyo galka mashruuca ee si cad loo furay ayaa la wadaagaa. Xaaladda soo galitaanka Claude ee martida loo yahay, boggaga browserka, iyo furayaasha SSH si toos ah looma soo dejiyo.
- **Eeg heerka shabakadaada** Maaraynta wakiilnimada shabakada ee ku dhex jirta ayaa kuu ogolaanaysa inaad soo dejiso rukunsiyada, dooro meelaha dhamaadka, oo aad hubiso IP-ga ka baxa. Deegaanku internetka kuma xirmayo ilaa hubinnada muhiimka ahi ay si guul leh u gudbaan. Haddii wakiillada shabakadu maqan tahay, ma dhacayso xiriir toos ah.
- **Sii wad marka xigta** Qalabka, faylalka, iyo xaaladda gelitaanku waxay ku sii jiraan saxanka casriga ah. Hawlaha asalka ahi waxay sii socon karaan markaad daaqadda xidhid.
- **Maamul akoonno badan oo Claude Code ah.** Eeg qoondada, u qoondee akoonno kulammada cusub, kuna beddel akoonka oggolaansho cad si aad taariikhda u sii wadato. [Claude Code](docs/guide/accounts.md)
- **Ku xidh shabakad gaar ah.** Gudaha Claude Eden ka gal Tailscale si martigeliyaha iyo deegaanku SSH ugu xidhmaan labada jiho. [Tailscale / SSH](docs/guide/tailscale.md)

## Bilaw degdeg ah

U dir codsigan wakiilkaaga AI (codex, dsh, workbuddy - malaha maaha Claude Code 😁):

```markdown
Soo qaado koodhkii ugu dambeeyay ee https://github.com/CarterLeeAlt/Claude-Eden, Windows u dhis oo rakib. U habee proxy-ga Claude Eden node-ka xx ee Clash-kayga maxalliga ah iyo IP-gayga xx. Marka hore akhri hagaha dhismaha, xaqiiji xadka rakibidda iyo dejinta shabakadda, dabadeed fulinta iyo natiijada hubi.
```

Waxa kaliya ee aad u baahan tahay waa barta ugu dambeysa ee wakiil shabakad la isku halayn karo. Mashruucani ma dalacsiinayo bixiye wakiil; Waxaad heli kartaa bixiye laftaada, tusaale ahaan adoo raadiya shabakada qaladka madow.

## Soo deji oo rakib

Martigeliyaha desktop-ka Claude Eden waa Windows 10 x64, martida maxalliga ahna waa Debian 13. Awoodaha nidaamka iyo dalwadeynta waxaa lagu ogaadaa awoodda dhabta ah ee kombiyuutar kasta. [Windows](docs/windows.md) · [Martida Debian 13](docs/guide/guest-systems.md).

1. Ka hubi rakibaha Windows ee la daabacay [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases). Haddii aan sii-deyn jirin, raac [hagaha dhismaha](docs/windows.md).
2. Orod rakibaha Windows oo bilow **Claude Eden**.
3. Raac tilmaamaha app-ka si aad u habayso wakiilkaaga shabakad, bilaw deegaanka, oo aad u furto mashruuc.

Rakibuhu wuxuu ka kooban yahay goobta shaqada iyo matoorka proxy-ga. Uma baahnid inaad horay u rakibto Go, Node.js, Docker ama Clash. Markaad deegaanka abuurayso, dooro goobta kaydinta Debian 13, dabadeed diyaari sawirka rasmiga ah iyo qalabka. Adiga ayaa diyaarsanaya adeegga proxy-ga iyo akoonka Claude.

Koodhka ishu waa dadweyne. Qoraallada noocyada la daabacay iyo checksum-yada ka eeg [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases).

Awooduhu waxay ku xiran yihiin build-ka la isticmaalayo; awoodaha iyo xadka tijaabada rakibaha dadweynaha waxay ku qoran yihiin qoraalka sii-deynta.

Windows saxeexan wuxuu ku cusboonaadaa proxy-ga la doortay. Dhismayaasha aan saxeexnayn gacanta ayaa loogu cusboonaysiiyaa rakibaha. Marka hore kaydi oo jooji hawlaha; xogta shakhsiga ah waa la hayaa. [Cusboonaysiin iyo saarid](docs/guide/maintenance.md)

## Ku bilow saddex tallaabo

### 1. Dooro wakiil shabakad

Bogga guriga, dhagsii 'Configure network and proxy', ka dibna 'Ku dar isha'. Soo rar isdiiwaangelinta Clash/Mihomo YAML ama faylka YAML maxalli ah, oo dooro dhammaadka hoosta 'Nodes'. Haddii aysan jirin wakiil shabakadeed, waxaad si cad u dooran kartaa 'Soo deji rukuntan iyada oo loo marayo shabakadda martida loo yahay', ama si toos ah u soo deji faylka maxalliga ah. Ka dib, maamul meelaha dhamaadka ee hoos yimaada 'Settings → Network and proxy'.

### 2. Bilow deegaanka

Marka ugu horreysa dooro goobta kaydinta Debian 13. Rakib martida, bilow deegaanka oo bixi oggolaanshaha loo baahan yahay. Horumarka iyo khaladaadku waxay ka muuqdaan faahfaahinta hawsha. Bilowga dambe wuxuu isticmaalaa isla deegaanka.

### 3. Fur mashruuc oo maamul Claude

Guji 'Mashruuca Furan' si aad u dooratid galka kombiyuutarkaaga, fur terminal cusub oo orod `claude`; dabadeed akoonkaaga ku xaqiiji biraawsarka Workbench. Waxa kale oo aad ka bilaabi kartaa goobta shaqada ee caadiga ah. Tallaabooyinka buuxa: [Bilaabidda](docs/getting-started.md).

## Fahmo xadkan ka hor intaanad isticmaalin

- Wax ka beddelka iyo tirtiridda mashaariicda la wadaago waxay si toos ah u saameeyaan faylasha kombiyuutarkaaga. Isticmaal xakamaynta nooca ama kayd. `.git` waa la akhriyi karaa lana qori karaa; Claude Eden wuxuu taageeraa stage, commit iyo beddelidda branches.
- Mashaariicda hal qaabayn Claude Eden ah waxay wadaagaan martida, qalabka, akoonnada iyo nidaamka faylka; iskama go'na. Beddelidda nidaamka waxay u baahan tahay rakibid gaar ah, xogta deegaankii horena waa la hayaa.
- Sida caadiga ah bixitaanka internetku wuxuu taageeraa IPv4 TCP, DNS-na isla proxy-ga ayuu maraa. UDP, IPv6 iyo ICMP dadweyne iyo xidhiidh toos ah oo martigeliye ama LAN ah waa xaddidan yihiin. Markaad si cad u hawlgeliso Tailscale, gelitaanka aaladuhu wuxuu raacaa oggolaanshaha shabakadda gaarka ah.
- Jeegaga shabakada waxay tusinaysaa qaabka hadda jira iyo cadaymaha la muunada ah. Ma dammaanad qaadi karaan in akoonku uu ka fogaanayo xannibaadaha goobta, mana beddelaan xisaab-xidhka amniga oo dhammaystiran.

Faahfaahinta, eeg [Xuduudaha shabakada iyo xogta](docs/getting-started.md#网络与数据边界).

## Dukumentiyada iyo wax ku biirinta

Si aad u hesho tusmada buuxda, eeg [Tusmada dhokumentiyada](docs/README.md).

| Hawshaada | Halkan ka bilow |
|---|---|
| Debian 13 | [Martida Debian 13](docs/guide/guest-systems.md) |
| Ku rakib, habee wakiil shabakad, oo fur mashruucaaga ugu horreeya | [Bilaabidda](docs/getting-started.md) |
| Xallinta soo dejinta, bilaabista, isku xidhka, iyo soo gelista | [FAQ iyo soo kabashada](docs/troubleshooting.md) |
| Baro kontaroolada kiiboodhka, muuqaalka, adkaysiga xogta, iyo habdhaqanka bixista | [Isticmaalka maalinlaha ah](docs/getting-started.md#日常使用) |
| Ka warbixi arrin dib loo soo saari karo | [GitHub Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues) |
| Ku biiri sixid, hawl cusub ama dukumenti: dib-u-eegista Issue → martiqaadka wada-shaqeeyaha → PR | [Hagaha wax-ku-biirinta (Shiine)](.github/CONTRIBUTING.md) |

Mashruucu wuu kobcayaa; khaladaadka iyo talooyinka ku soo gudbi kaydkan.

## Asalka iyo shatiga

Claude Eden wuxuu ku salaysan yahay [Claude-Cage](https://github.com/Lee-xeo/Claude-Cage) iyo [agent-vm](https://github.com/sylvinus/agent-vm); asalka iyo ruqsadihii hore waa la hayaa.

Shatiga ugu weyn ee mashruuca waa [MIT](LICENSE). Qaybaha xidhxidhan waxay haystaan ​​shatiyadooda. Mihomo waxaa loo qaybiyaa sidii mid gaar ah oo la fulin karo iyadoo hoos timaada GPL-3.0. Sidoo kale arag [NOTICE](NOTICE) · [Shatiyada xisbiga saddexaad](THIRD_PARTY_LICENSES) · [noVNC](ui/vendor/novnc/LICENSE.txt)。

Claude Eden waa mashruuc bulsho oo madax bannaan oo aan xidhiidh la lahayn Anthropic.

## Tix-raac iyo mahadnaq

Waad ku mahadsan tahay mashaariicda soo socota ee kartida aasaasiga ah iyo tixraacyada naqshadeynta:

- [agent-vm](https://github.com/sylvinus/agent-vm): aasaaska mashruuca.
- [Lima](https://github.com/lima-vm/lima): Maareynta mashiinka farsamada ee Linux.
- [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock): isku xirka mashiinka farsamada.
- [sshocker](https://github.com/lima-vm/sshocker) iyo [SFTP](https://github.com/pkg/sftp): wadaagista faylka u dhexeeya mashiinka farsamada iyo martigeliyaha.
- [noVNC](https://github.com/novnc/noVNC): soo bandhigida fog iyo isdhexgalka browserka.
- [Mihomo](https://github.com/MetaCubeX/mihomo):-ku-xidhan xudunta wakiilnimada.
- [code-server](https://github.com/coder/code-server): tifaftiraha koodhka ku salaysan browserka.
- [AIO Sandbox](https://github.com/agent-infra/sandbox): tixraacyada jawiga shaqada iyo naqshadeynta isdhexgalka.
- [claude-switcher](https://github.com/YOURSJEYO/claude-switcher) iyo [Claude-Guardian](https://github.com/DUNHKpcc/Claude-Guardian): tixraacyada hubinta shabakada iyo ogaanshaha amniga.
- [claude-post-ban-cleanup](https://github.com/l2461090/claude-post-ban-cleanup): tixraac loogu talagalay tirakoob sax ah, ilaalinta xogta iyo nadiifinta kaydka oo dib loo soo celin karo; Claude Eden ayaa si madax-bannaan u hirgeliyey.


<a href="https://www.star-history.com/?repos=CarterLeeAlt%2FClaude-Eden&amp;type=date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date&amp;theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" />
    <img alt="Claude Eden GitHub Star History" src="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" width="800" />
  </picture>
</a>
