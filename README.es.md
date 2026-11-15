<div align="center">

<h1>Claude Eden</h1>

**Mantenga a la señorita Claude en una jaula para que podamos proteger mejor nuestro tiempo juntos.**

<img src="assets/claude-eden-character.png" alt="Claude Eden" width="320" />

[![Release](https://img.shields.io/github/v/release/CarterLeeAlt/Claude-Eden?include_prereleases)](https://github.com/CarterLeeAlt/Claude-Eden/releases) [![CI](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/CarterLeeAlt/Claude-Eden/actions/workflows/ci.yml) [![MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

**[Proyecto](https://github.com/CarterLeeAlt/Claude-Eden) · [Versiones](https://github.com/CarterLeeAlt/Claude-Eden/releases) · [Empezando](docs/getting-started.md) · [Preguntas frecuentes](docs/troubleshooting.md) · [Guía de uso (chino)](docs/guide/index.md)**

[简体中文](README.md) · [繁體中文](README.zh-TW.md) · [Русский](README.ru.md) · [العربية](README.ar.md) · [فارسی](README.fa.md) · [한국어](README.ko.md) · [Español](README.es.md) · [မြန်မာဘာသာ](README.my.md) · [Беларуская](README.be.md) · [پښتو](README.ps.md) · [دری](README.prs.md) · [አማርኛ](README.am.md) · [Soomaali](README.so.md) · [ትግርኛ](README.ti.md) · [Français](README.fr.md)

</div>

## Hartazgo con Anthropic

Como usuarios que sólo queremos hablar y colaborar con Claude, estamos cansados de configurar VPS, perfiles del navegador y comprobaciones del entorno mientras nos preocupamos por los bloqueos de cuentas y las pérdidas financieras. Presentamos **Claude Eden** — **un espacio de trabajo de escritorio: configure su propio proxy y administre su entorno aislado, zona horaria, idioma y comprobaciones en un solo lugar.**

Claude Eden reúne terminal, navegador, editor y Claude Desktop oficial en Windows 10 con un invitado Debian 13 local. Comparte sólo los proyectos que elijas y gestiona la red por separado. Las capacidades del host y los recursos se detectan en cada equipo. Sus funciones principales:

- Configuraciones de red aisladas del host; Claude Eden enruta todo el tráfico de red dentro de su entorno a través de su propio proxy de red.
- Configuración de huellas digitales del navegador aislada del host, incluida la zona horaria y el idioma, con sincronización automática con la ubicación IP del proxy de red
- Integración en todo el flujo de trabajo de Claude: inicio de sesión en el navegador, autorización de cuenta y proceso de Claude Code
- Las comprobaciones de red y aislamiento integradas pausan el acceso a Internet cuando las comprobaciones críticas fallan o la evidencia está incompleta; Los controles comunitarios y de terceros proporcionan información de referencia adicional.

## ¿Qué puedes hacer con Claude Eden?

- **Trabaja en una sola ventana.** Ejecute Claude Code en la terminal, revise los cambios en el editor y use el navegador Workbench para obtener una vista previa de los servicios e iniciar sesión.
- **Desarrolla en un entorno Linux independiente.** Utiliza herramientas Linux en Debian 13 local y comparte solo los proyectos que autorices explícitamente. [Invitado Debian 13](docs/guide/guest-systems.md)
- **Controla lo que compartes.** Solo se comparten el espacio de trabajo predeterminado y las carpetas de proyectos abiertas explícitamente. El estado de inicio de sesión de Claude, los perfiles del navegador y las claves SSH del host no se importan automáticamente.
- **Vea el estado de su red.** La administración de proxy de red incorporada le permite importar suscripciones, seleccionar puntos finales y verificar la IP de salida. El entorno permanece fuera de línea hasta que se superan las comprobaciones críticas. Si el proxy de red no está disponible, no se recurre a una conexión directa como alternativa.
- **Continúe la próxima vez.** Las herramientas, los archivos y el estado de inicio de sesión persisten en el disco virtual. Las tareas en segundo plano pueden seguir ejecutándose cuando cierras la ventana.
- **Gestione varias cuentas de Claude Code.** Consulta cuotas, asigna cuentas a nuevas sesiones y, con autorización explícita, cambia de cuenta para continuar el historial. [Claude Code](docs/guide/accounts.md)
- **Conecta por una red privada.** Inicia sesión en Tailscale dentro de Claude Eden para conectar el equipo y el entorno por SSH en ambos sentidos. [Tailscale / SSH](docs/guide/tailscale.md)

## Inicio rápido

Envíe esta solicitud a su agente de IA (codex, dsh, workbuddy; probablemente no Claude Code 😁):

```markdown
Obtén el código fuente más reciente de https://github.com/CarterLeeAlt/Claude-Eden, compílalo para Windows e instálalo. Configura el proxy de Claude Eden para usar el nodo xx de mi Clash local y salir por mi IP xx. Primero lee las instrucciones de compilación y confirma el alcance de instalación y la configuración de red; después ejecuta y verifica el resultado.
```

Todo lo que necesita es un punto final de proxy de red confiable. Este proyecto no promueve ningún proveedor de proxy; Puede encontrar un proveedor usted mismo, por ejemplo buscando en el sitio web de Black Error.

## Descarga e instalación

El host de escritorio de Claude Eden es Windows 10 x64 y el invitado local es Debian 13. Las capacidades del sistema y de virtualización se detectan según las características reales de cada equipo. [Windows](docs/windows.md) · [Invitado Debian 13](docs/guide/guest-systems.md).

1. Consulte los instaladores Windows publicados en [GitHub Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases). Si no hay versiones publicadas, siga la [guía de compilación](docs/windows.md).
2. Ejecute el instalador Windows e inicie **Claude Eden**.
3. Siga las instrucciones de la aplicación para configurar su propio proxy de red, iniciar el entorno y abrir un proyecto.

El instalador incluye el espacio de trabajo y el motor del proxy. No necesitas instalar antes Go, Node.js, Docker ni Clash. Al crear el entorno, elige la ubicación de Debian 13 y prepara la imagen oficial y las herramientas. El servicio de proxy y la cuenta Claude los aporta el usuario.

El código fuente es público. Las notas y sumas de comprobación de las versiones publicadas están en [Releases](https://github.com/CarterLeeAlt/Claude-Eden/releases).

Las funciones dependen de la compilación; las capacidades y verificaciones del instalador público figuran en las notas de la versión.

Las versiones firmadas de Windows se actualizan mediante el proxy elegido. Las compilaciones sin firma se actualizan manualmente con el instalador. Guarda y detén las tareas antes de actualizar; los datos personales se conservan. [Actualizar y desinstalar](docs/guide/maintenance.md)

## Comience en tres pasos

### 1. Seleccione un proxy de red

En la página de inicio, haga clic en "Configurar red y proxy" y luego en "Agregar fuente". Importe una suscripción YAML de Clash/Mihomo o un archivo YAML local y seleccione un punto final en "Nodos". Si no hay ningún proxy de red disponible, puede seleccionar explícitamente "Descargar esta suscripción a través de la red de host" o importar un archivo local directamente. Luego, administre los puntos finales en 'Configuración → Red y proxy'.

### 2. Iniciar el entorno

Al crear el entorno, elige la ubicación de almacenamiento de Debian 13. Instala el invitado, inicia el entorno y completa las autorizaciones. El progreso y los errores aparecen en los detalles de la operación. Los siguientes inicios reutilizan el mismo entorno.

### 3. Abra un proyecto y ejecute Claude.

Haga clic en 'Abrir proyecto' para seleccionar una carpeta en su equipo, abra una nueva terminal e ingrese `claude`; luego autorice su propia cuenta en el navegador Workbench. También puedes comenzar con el espacio de trabajo predeterminado sin preparar un proyecto. Para conocer los pasos completos, consulte [Empezando](docs/getting-started.md).

## Comprenda estos límites antes de usarlo

- Las ediciones y eliminaciones en proyectos compartidos afectan directamente a los archivos de su equipo. Utilice control de versiones o copias de seguridad. El directorio `.git` permite lectura y escritura; puede preparar cambios, crear commits y cambiar de rama en Claude Eden.
- Los proyectos de una misma configuración de Claude Eden comparten el invitado, las herramientas, las cuentas y el sistema de archivos. No están aislados entre sí. Cambiar de sistema requiere una instalación independiente; los datos del entorno anterior se conservan.
- Por defecto, las conexiones públicas salientes admiten IPv4 TCP y DNS usa el mismo proxy. UDP, IPv6, ICMP públicos y el acceso directo al equipo o a la red local están limitados. Al activar Tailscale, el acceso a los dispositivos sigue los permisos de la red privada.
- Las comprobaciones de red muestran la configuración actual y pruebas de muestra. No pueden garantizar que una cuenta evite las restricciones de la plataforma y no reemplazan una auditoría completa de seguridad de virtualización.

Para más detalles, consulte [Límites de red y datos](docs/getting-started.md#网络与数据边界).

## Documentación y aportaciones

Para consultar el índice completo, consulte [Índice de documentación](docs/README.md).

| tu tarea | Empieza aquí |
|---|---|
| Debian 13 | [Invitado Debian 13](docs/guide/guest-systems.md) |
| Instale, configure un proxy de red y abra su primer proyecto | [Empezando](docs/getting-started.md) |
| Solucionar problemas de descargas, inicio, conectividad e inicio de sesión | [Preguntas frecuentes y recuperación](docs/troubleshooting.md) |
| Aprenda los controles del teclado, la apariencia, la persistencia de datos y el comportamiento de salida | [Uso diario](docs/getting-started.md#日常使用) |
| Informar un problema reproducible | [GitHub Issues](https://github.com/CarterLeeAlt/Claude-Eden/issues) |
| Contribuir correcciones, funciones o documentación: revisión del Issue → invitación como colaborador → PR | [Guía de contribución (chino)](.github/CONTRIBUTING.md) |

El proyecto sigue evolucionando. Comparta problemas y propuestas de mejora en este repositorio.

## Orígenes y licencias

Claude Eden se basa en [Claude-Cage](https://github.com/Lee-xeo/Claude-Cage) y [agent-vm](https://github.com/sylvinus/agent-vm), conservando sus atribuciones y licencias originales.

La licencia principal del proyecto es [MIT](LICENSE). Los componentes incluidos conservan sus respectivas licencias. Mihomo se distribuye como un ejecutable independiente bajo GPL-3.0. Ver también [NOTICE](NOTICE) · [Licencias de terceros](THIRD_PARTY_LICENSES) · [noVNC](ui/vendor/novnc/LICENSE.txt)。

Claude Eden es un proyecto comunitario independiente y no está afiliado a Anthropic.

## Referencias y agradecimientos

Gracias a los siguientes proyectos por capacidades fundamentales y referencias de diseño:

- [agent-vm](https://github.com/sylvinus/agent-vm): la base del proyecto.
- [Lima](https://github.com/lima-vm/lima): Gestión de máquinas virtuales Linux.
- [gvisor-tap-vsock](https://github.com/containers/gvisor-tap-vsock): redes de máquinas virtuales.
- [sshocker](https://github.com/lima-vm/sshocker) y [SFTP](https://github.com/pkg/sftp): intercambio de archivos entre la máquina virtual y el host.
- [noVNC](https://github.com/novnc/noVNC): visualización remota e interacción en el navegador.
- [Mihomo](https://github.com/MetaCubeX/mihomo): el núcleo de proxy de red integrado.
- [code-server](https://github.com/coder/code-server): el editor de código basado en navegador.
- [AIO Sandbox](https://github.com/agent-infra/sandbox): referencias para el ambiente de trabajo y el diseño de interacción.
- [claude-switcher](https://github.com/YOURSJEYO/claude-switcher) y [Claude-Guardian](https://github.com/DUNHKpcc/Claude-Guardian): referencias para comprobaciones de red y diagnósticos de seguridad.
- [claude-post-ban-cleanup](https://github.com/l2461090/claude-post-ban-cleanup): referencia para el inventario preciso, la protección de datos y la limpieza recuperable de caché; implementación independiente en Claude Eden.


<a href="https://www.star-history.com/?repos=CarterLeeAlt%2FClaude-Eden&amp;type=date">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date&amp;theme=dark" />
    <source media="(prefers-color-scheme: light)" srcset="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" />
    <img alt="Claude Eden GitHub Star History" src="https://api.star-history.com/svg?repos=CarterLeeAlt/Claude-Eden&amp;type=Date" width="800" />
  </picture>
</a>
