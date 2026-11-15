<div align="center">

<h1>Claude Eden</h1>

**Gardez Miss Claude dans une cage pour que nous puissions mieux protéger notre temps ensemble**

<img src="assets/claude-eden-character.png" alt="Claude Eden" width="320" />

[![Release](https://img.shields.io/github/v/release/CarterLeeAlt/Claude-Eden?include_prereleases)](https://github.com/CarterLeeAlt/Claude-Eden/releases) [![CI](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**[Projet](https://github.com/CarterLeeAlt/Claude-Eden) · [Versions publiées](https://github.com/CarterLeeAlt/Claude-Eden/releases) · [Commencer](docs/getting-started.md) · [FAQ](docs/troubleshooting.md) · [Guide d’utilisation (chinois)](docs/guide/index.md)**

[简体中文](README.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [العربية](README.ar.md) · [فارسی](README.fa.md) · [한국어](README.ko.md) · [Español](README.es.md) · [မြန်မာဘာသာ](README.my.md) · [Беларуская](README.be.md) · [پښتو](README.ps.md) · [دری](README.prs.md) · [አማርኛ](README.am.md) · [Soomaali](README.so.md) · [ትግርኛ](README.ti.md) · [Français](README.fr.md)

</div>

## Assez d’Anthropic

En tant qu'utilisateurs ordinaires qui souhaitent simplement parler et collaborer avec Claude, nous en avons assez de configurer des instances VPS, des profils d'empreintes digitales de navigateur et des contrôles d'environnement, tout en nous inquiétant des interdictions de compte et des pertes financières. Nous présentons **Claude Eden** — **une application de bureau : configurez votre propre proxy réseau et gérez votre environnement isolé, votre fuseau horaire, votre langue et vos contrôles d'environnement en un seul endroit.**

Claude Eden réunit terminal, navigateur, éditeur et Claude Desktop officiel dans un même espace de travail. L’application Windows utilise des invités Linux ou Windows selon les capacités de l’hôte, ou un environnement cloud Linux par SSH. Partagez les projets de votre choix, gérez le réseau séparément et poursuivez vos tâches depuis votre téléphone. Ses principales fonctions :

- Paramètres réseau isolés de l'hôte ; Claude Eden achemine le trafic de son environnement via son propre proxy réseau
- Paramètres d'empreinte digitale du navigateur isolés de l'hôte, y compris le fuseau horaire et la langue, avec synchronisation automatique avec l'emplacement IP du proxy réseau
- Intégration tout au long du flux de travail Claude : connexion au navigateur, autorisation de compte et processus Claude Code
- Les contrôles de réseau et d'isolement intégrés interrompent l'accès à Internet lorsque les contrôles critiques échouent ou que les preuves sont incomplètes ; les contrôles communautaires et tiers fournissent des informations de référence supplémentaires

## Ce que vous pouvez faire avec Claude Eden

- **Travaillez dans une seule fenêtre.** Exécutez Claude Code dans le terminal, examinez les modifications dans l'éditeur et utilisez le navigateur Workbench pour prévisualiser les services et vous connecter.
- **Développez dans votre système habituel.** Choisissez un invité Windows ou Linux avec ses outils ; les hôtes Windows compatibles proposent aussi une installation adaptée à leur version. [Choisir le système invité](docs/guide/guest-systems.md)
- **Contrôlez ce que vous partagez.** Seuls l'espace de travail par défaut et les dossiers de projet explicitement ouverts sont partagés. L'état de connexion Claude de l'hôte, les profils de navigateur et les clés SSH ne sont pas importés automatiquement.
- **Consultez l'état de votre réseau.** La gestion intégrée du proxy réseau vous permet d'importer des abonnements, de sélectionner des points de terminaison et de vérifier l'adresse IP de sortie. L'environnement reste hors ligne jusqu'à ce que les vérifications critiques soient réussies. Si le proxy réseau n'est pas disponible, il n'est pas possible de recourir à une connexion directe.
- **Continuez la prochaine fois.** Les outils, les fichiers et l'état de connexion persistent sur le disque virtuel. Les tâches en arrière-plan peuvent continuer à s'exécuter lorsque vous fermez la fenêtre.
- **Gérez plusieurs comptes Claude Code.** Consultez les quotas, attribuez les comptes aux nouvelles sessions et, avec une autorisation explicite, changez de compte en conservant l’historique. [Claude Code](docs/guide/accounts.md)
- **Poursuivez depuis votre téléphone.** Associez votre téléphone Android à l’ordinateur par QR code ou code d’association. Retrouvez les mêmes conversations Claude Code, envoyez des messages, approuvez les outils et utilisez le navigateur distant ou la fenêtre Claude Desktop de l’ordinateur.
- **Exécutez Claude Eden dans le cloud.** Connectez-vous par SSH à un VPS Linux x86_64 compatible, sans KVM. Les données et la sortie réseau restent sur le serveur.
- **Connectez-vous par un réseau privé.** Connectez Tailscale dans Claude Eden pour accéder par SSH à l’hôte et à l’environnement dans les deux sens. [Tailscale / SSH](docs/guide/tailscale.md)

## Démarrage rapide

Envoyez cette demande à votre agent IA (codex, dsh, workbuddy — probablement pas Claude Code 😁) :

```markdown
Récupère le code source de https://github.com/CarterLeeAlt/Claude-Eden, lis les instructions de construction Windows, puis compile et installe l’application. Configure son proxy pour utiliser le nœud xx de mon Clash local et sortir par mon IP xx. Vérifie le périmètre d’installation et la configuration avant l’exécution, puis valide le résultat.
```

Tout ce dont vous avez besoin est un point de terminaison proxy réseau fiable. Ce projet ne fait la promotion d'aucun fournisseur de proxy ; vous pouvez trouver un fournisseur vous-même, par exemple en recherchant le site Web Black Error.

## Téléchargement et installation

L’hôte de bureau est Windows 10 22H2 / 11 x64, avec Windows 10 comme plateforme principale de validation. Les invités Linux restent disponibles ; l’invité Windows natif exige actuellement Windows 11 Pro, Entreprise ou Éducation avec Hyper-V. Le déploiement cloud Linux est conservé. [Windows](docs/windows.md) · [Cloud Linux](docs/linux.md).

1. Consultez les paquets Windows réellement disponibles dans [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases), ou les instructions de construction si aucun paquet n’est publié.
2. Exécutez l’installateur Windows et ouvrez **Claude Eden**. [Guide d’installation](docs/windows.md#安装并开始工作) · [Cloud Linux](docs/guide/cloud.md).
3. Suivez les instructions de l'application pour configurer votre propre proxy réseau, démarrer l'environnement et ouvrir un projet.

L’installateur comprend l’espace de travail et le moteur de proxy. Inutile d’installer Go, Node.js, Docker ou Clash au préalable. À la création, choisissez le système invité et son emplacement, puis préparez l’image officielle et les outils. Claude Eden ne fournit ni licence de système, ni service proxy, ni compte Claude.

Le code source est public. Les paquets publiés, leurs notes et leurs sommes de contrôle figurent dans [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases).

Les fonctions dépendent de la compilation utilisée ; les fonctions et les vérifications de l’installateur public sont précisées dans les notes de version.

Les versions Windows signées se mettent à jour via le proxy choisi. Les versions de développement non signées utilisent l’installateur manuellement. Enregistrez et arrêtez les tâches avant la mise à jour ; les données personnelles sont conservées. [Mise à jour et désinstallation](docs/guide/maintenance.md)

## Commencez en trois étapes

### 1. Sélectionnez un proxy réseau

Sur la page d'accueil, cliquez sur « Configurer le réseau et le proxy », puis sur « Ajouter une source ». Importez un abonnement YAML Clash/Mihomo ou un fichier YAML local et sélectionnez un point de terminaison sous « Nœuds ». Si aucun proxy réseau n'est disponible, vous pouvez sélectionner explicitement « Télécharger cet abonnement via le réseau hôte » ou importer directement un fichier local. Plus tard, gérez les points de terminaison sous « Paramètres → Réseau et proxy ».

### 2. Démarrez l'environnement

À la première création, choisissez le système invité et le stockage. Démarrez l’environnement Claude Eden et suivez l’installation et les autorisations. La progression et les erreurs figurent dans les détails de l’opération. Les lancements suivants réutilisent le même environnement.

### 3. Ouvrez un projet et exécutez Claude

Cliquez sur « Ouvrir le projet » pour sélectionner un dossier sur votre ordinateur, ouvrez un nouveau terminal et entrez `claude`; puis autorisez votre propre compte dans le navigateur Workbench. Vous pouvez également commencer avec l'espace de travail par défaut sans préparer de projet. Pour les étapes complètes, voir [Commencer](docs/getting-started.md).

## Comprendre ces limites avant utilisation

- Les modifications et suppressions dans les projets partagés affectent directement les fichiers sur votre ordinateur. Utilisez le contrôle de version ou les sauvegardes. Le dossier `.git` du projet est accessible en lecture et en écriture ; vous pouvez préparer des modifications, créer des commits et changer de branche dans Claude Eden.
- Tous les projets d’un profil partagent l’environnement sélectionné, ses outils, ses comptes et son système de fichiers Windows ou Linux. Les projets ne sont pas isolés les uns des autres.
- Par défaut, les sorties publiques prennent en charge IPv4 TCP et DNS passe par le même proxy. UDP, IPv6, ICMP publics et les accès directs à l’hôte ou au réseau local sont limités. En activant Tailscale, les accès aux appareils suivent les règles du réseau privé.
- Les vérifications du réseau montrent la configuration actuelle et des échantillons de preuves. Ils ne peuvent pas garantir qu’un compte évitera les restrictions de la plateforme et ne remplacent pas un audit complet de sécurité de la virtualisation.

Pour plus de détails, voir [Limites du réseau et des données](docs/getting-started.md#网络与数据边界).

## Documentation et contributions

Pour l'index complet, voir [Index des documents](docs/README.md).

| Votre tâche | Commencez ici |
|---|---|
| Windows / Debian / Ubuntu 22.04 | [Choisir le système invité](docs/guide/guest-systems.md) |
| Installez, configurez un proxy réseau et ouvrez votre premier projet | [Commencer](docs/getting-started.md) |
| Dépanner les téléchargements, le démarrage, la connectivité et la connexion | [FAQ et récupération](docs/troubleshooting.md) |
| Apprenez les commandes du clavier, l'apparence, la persistance des données et le comportement de sortie | [Utilisation quotidienne](docs/getting-started.md#日常使用) |
| Signaler un problème reproductible | [GitHub Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues) |
| Contribuer un correctif, une fonction ou de la documentation : examen de l’Issue → invitation comme collaborateur → PR | [Guide de contribution (chinois)](.github/CONTRIBUTING.md) |

Le projet évolue. Vos retours et contributions sont les bienvenus dans ce dépôt.

## Origines et licences

Claude Eden est dérivé de [Claude-Cage](https://github.com/Lee-xeo/Claude-Cage) et de [agent-vm](https://github.com/sylvinus/agent-vm). Les licences et attributions d’origine sont conservées.

La licence principale du projet est [MIT](LICENSE). Les composants groupés conservent leurs licences respectives. Mihomo est distribué sous forme d'exécutable distinct sous GPL-3.0. Voir aussi [NOTICE](NOTICE) · [Licences tierces](THIRD_PARTY_LICENSES) · [noVNC](ui/vendor/novnc/LICENSE.txt)。

Claude Eden est un projet communautaire indépendant et n'est pas affilié à Anthropic.

## Références et remerciements

Merci aux projets suivants pour leurs capacités fondamentales et leurs références de conception :

- [agent-vm](https://github.com/sylvinus/agent-vm): la fondation amont de Claude Eden.
- [Lima](https://github.com/lima-vm/lima): Gestion des machines virtuelles Linux.
- [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock): mise en réseau de machines virtuelles.
- [sshocker](https://github.com/lima-vm/sshocker) et [SFTP](https://github.com/pkg/sftp): partage de fichiers entre la machine virtuelle et l'hôte.
- [noVNC](https://github.com/novnc/noVNC): affichage à distance et interaction dans le navigateur.
- [Mihomo](https://github.com/MetaCubeX/mihomo): le noyau proxy réseau intégré.
- [code-server](https://github.com/coder/code-server): l'éditeur de code basé sur un navigateur.
- [AIO Sandbox](https://github.com/agent-infra/sandbox): références pour l'environnement de travail et le design d'interaction.
- [claude-switcher](https://github.com/YOURSJEYO/claude-switcher) et [Claude-Guardian](https://github.com/DUNHKpcc/Claude-Guardian): références pour les vérifications du réseau et les diagnostics de sécurité.
- [claude-post-ban-cleanup](https://github.com/l2461090/claude-post-ban-cleanup): référence pour l’inventaire précis, la protection des données et le nettoyage réversible du cache ; implémentation indépendante dans Claude Eden.


<a href="https://www.star-history.com/?repos=CarterLeeAlt%2FClaude-Eden&amp;type=date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date&amp;theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" />
    <img alt="Claude Eden GitHub Star History" src="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" width="800" />
  </picture>
</a>
