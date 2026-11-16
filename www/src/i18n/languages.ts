export const languages = [
  {
    code: 'zh', name: '简体中文', lang: 'zh-CN', dir: 'ltr',
    languageLabel: '语言', downloadLabel: "下载应用", platformLabel: "选择下载平台", closeLabel: "关闭", installGuideLabel: "安装说明", unavailableLabel: "尚无公开安装包", unsignedLabel: "预览版 · 未签名",
    checksLabel: '网络与隔离检查',
    firstLookLabel: '首次启动界面', releaseNotesLabel: '版本说明与校验文件', issuesLabel: '问题反馈',
  },
  {
    code: 'zh-TW', name: '繁體中文', lang: 'zh-TW', dir: 'ltr',
    languageLabel: '語言', downloadLabel: "下載應用程式", platformLabel: "選擇下載平台", closeLabel: "關閉", installGuideLabel: "安裝說明", unavailableLabel: "尚無公開安裝包", unsignedLabel: "預覽版 · 未簽章",
    checksLabel: '網路與隔離檢查',
    firstLookLabel: '首次啟動畫面', releaseNotesLabel: '版本說明與校驗檔案', issuesLabel: '問題回報',
  },
  {
    code: 'en', name: 'English', lang: 'en', dir: 'ltr',
    languageLabel: 'Language', downloadLabel: 'Download the app', platformLabel: 'Choose your platform', closeLabel: 'Close', installGuideLabel: 'Installation guide', unavailableLabel: 'No public installer yet', unsignedLabel: 'Preview · unsigned',
    checksLabel: 'Network and isolation checks',
    firstLookLabel: 'First launch', releaseNotesLabel: 'Release notes and checksums', issuesLabel: 'Report an issue',
  },
  {
    code: 'ja', name: '日本語', lang: 'ja', dir: 'ltr',
    languageLabel: '言語', downloadLabel: 'ダウンロード', platformLabel: 'プラットフォームを選択', closeLabel: '閉じる', installGuideLabel: 'インストール手順', unavailableLabel: '公開インストーラーはまだありません', unsignedLabel: 'プレビュー版・署名なし',
    checksLabel: 'ネットワークと分離の検査',
    firstLookLabel: '初回起動の画面', releaseNotesLabel: 'リリースノートとチェックサム', issuesLabel: '問題を報告',
  },
  {
    code: 'ko', name: '한국어', lang: 'ko', dir: 'ltr',
    languageLabel: '언어', downloadLabel: "앱 다운로드", platformLabel: "플랫폼 선택", closeLabel: "닫기", installGuideLabel: "설치 안내", unavailableLabel: "공개 설치 패키지 없음", unsignedLabel: "미리 보기 · 서명 없음",
    checksLabel: '네트워크 및 격리 검사',
    firstLookLabel: '첫 실행 화면', releaseNotesLabel: '릴리스 노트 및 체크섬 파일', issuesLabel: '문제 제보',
  },
  {
    code: 'de', name: 'Deutsch', lang: 'de', dir: 'ltr',
    languageLabel: 'Sprache', downloadLabel: 'App herunterladen', platformLabel: 'Plattform auswählen', closeLabel: 'Schließen', installGuideLabel: 'Installationsanleitung', unavailableLabel: 'Noch kein öffentliches Installationspaket', unsignedLabel: 'Vorschau · unsigniert',
    checksLabel: 'Netzwerk- und Isolationsprüfungen',
    firstLookLabel: 'Erster Start', releaseNotesLabel: 'Versionshinweise und Prüfsummen', issuesLabel: 'Problem melden',
  },
  {
    code: 'fr', name: 'Français', lang: 'fr', dir: 'ltr',
    languageLabel: 'Langue', downloadLabel: "Télécharger l’application", platformLabel: "Choisissez votre plateforme", closeLabel: "Fermer", installGuideLabel: "Guide d’installation", unavailableLabel: "Paquet pas encore publié", unsignedLabel: "Version préliminaire · non signée",
    checksLabel: 'Contrôles du réseau et de l’isolation',
    firstLookLabel: 'Écran du premier démarrage', releaseNotesLabel: 'Notes de version et fichiers de sommes de contrôle', issuesLabel: 'Signaler un problème',
  },
] as const;

export type EdenLocale = typeof languages[number]['code'];
