<p align="center" style="margin-bottom: 0;">
  <img src=".github/assets/skillshare-logo-card.png" alt="skillshare" width="280">
</p>

<h1 align="center" style="margin-top: 0.5rem; margin-bottom: 0.5rem;">skillshare</h1>

<p align="center">
  <a href="README.md">English</a> · <a href="README-de.md">Deutsch</a> · <a href="README-es.md">Español</a> · <a href="README-fr.md">Français</a> · <a href="README-ja.md">日本語</a> · <a href="README-ko.md">한국어</a> · <a href="README-pt-BR.md">Português</a> · <a href="README-tr.md">Türkçe</a> · <a href="README-zh-CN.md">简体中文</a> · <a href="README-zh-TW.md">繁體中文</a>
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc"><img src="https://img.shields.io/badge/Website-skillshare.runkids.cc-blue?logo=docusaurus" alt="Website"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-MIT-yellow.svg" alt="License: MIT"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/v/release/runkids/skillshare" alt="Release"></a>
  <a href="https://github.com/runkids/skillshare/releases"><img src="https://img.shields.io/github/downloads/runkids/skillshare/total" alt="Downloads"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-blue" alt="Platform">
  <a href="https://deepwiki.com/runkids/skillshare"><img src="https://img.shields.io/badge/Ask-DeepWiki-blue" alt="Ask DeepWiki"></a>
  <a href="https://ko-fi.com/williehung"><img src="https://img.shields.io/badge/Support-skillshare-FF5E5B?logo=kofi&logoColor=white" alt="Support skillshare on Ko-fi"></a>
</p>

<p align="center">
  <a href="https://github.com/runkids/skillshare/stargazers"><img src="https://img.shields.io/github/stars/runkids/skillshare?style=social" alt="Star on GitHub"></a>
</p>

<p align="center">
  <a href="https://trendshift.io/repositories/21835" target="_blank"><img src="https://trendshift.io/api/badge/repositories/21835" alt="runkids%2Fskillshare | Trendshift" style="width: 250px; height: 55px;" width="250" height="55"/></a>
  <a href="https://trendshift.io/repositories/21835?utm_source=trendshift-badge&amp;utm_medium=badge&amp;utm_campaign=badge-trendshift-21835" target="_blank" rel="noopener noreferrer"><img src="https://trendshift.io/api/badge/trendshift/repositories/21835/daily?language=Go" alt="runkids%2Fskillshare | Trendshift" width="250" height="55"/></a>
</p>

<p align="center">
  <strong>Yapay zekâ kodlama kurulumunuz, her yerde.</strong><br>
  Becerileri, ajanları, kuralları, MCP bağlantılarını ve kancaları (hooks) tek bir yerde yönetin.<br>
  Claude Code, Codex, Pi, OpenCode ve daha fazlası için.
</p>

<p align="center">
  <a href="https://skillshare.runkids.cc">Web Sitesi</a> •
  <a href="#kurulum">Kurulum</a> •
  <a href="#hızlı-başlangıç">Hızlı Başlangıç</a> •
  <a href="#öne-çıkanlar">Öne Çıkanlar</a> •
  <a href="#screenshots">Ekran Görüntüleri</a> •
  <a href="#desktop-app">Masaüstü Uygulaması</a> •
  <a href="https://skillshare.runkids.cc/docs">Belgeler</a>
</p>

<p align="center">
  <img src=".github/assets/demo.gif" alt="skillshare demo" width="960">
</p>

> [!NOTE]
> **Son sürüm**: v0.34.0 — Herdr modları artık birer plugin: `herdr-app` hedefi Herdr modlarını ve başlatıcılarını kurar, günceller ve kaldırır; bir modun paneli mevcut düzeninizin bir kopyasında açılabilir ve kontrol paneli birden fazla plugin üzerinde aynı anda işlem yapar. `skillshare hooks sync NAME1 NAME2 --replace` çakışan birden fazla yerel hook'u tek seferde devralır; aynı ada sahip iki skill de artık atlanmak yerine senkronize edilir, biri tam yol adıyla. Özel tema **Glass: Everywhere**, altı yeni arka plan tonu ve yüzey seçenekleri ekler; kontrol paneli bir Agent'ın MCP dosyasını sayfada gösterebilir. [Herdr kılavuzu →](https://skillshare.runkids.cc/docs/learn/with-herdr) · [Tüm sürümler →](https://github.com/runkids/skillshare/releases)

## Neden skillshare

Yapay zekâ araçlarını değiştirmek, kurulumunuzu baştan oluşturmak anlamına gelmemelidir.
skillshare, becerilerinize ve diğer yapay zekâ kaynaklarınıza kontrol ettiğiniz bir yuva sunar.

- **Araçları değiştirin, becerilerinizi koruyun** — bir kez düzenleyin, ardından Claude Code, Codex, Pi ve kullandığınız diğer araçlarla eşitleyin.
- **Kurulumunuzu yanınızda taşıyın** — kaynağınızı Git'te sürümleyin ve başka bir makineye çekin.
- **Ekibinizle paylaşın** — proje kaynaklarını kodunuzla birlikte tutun ve paylaşılan becerileri izlenen depolar üzerinden dağıtın.

Bir ekip arkadaşınız Claude Code, diğeri Codex kullanıyor olabilir. Paylaşılan kod inceleme kontrol listenizi projenin yanında `.skillshare/` içinde tutun. Yeni ekip üyeleri sohbetten talimat kopyalamak yerine bildirilen becerileri kurar ve yapılandırılan araçlarla eşitler. [Ekip başlangıç rehberi →](https://skillshare.runkids.cc/docs/how-to/recipes/team-onboarding-recipe)

Her şeyi yerel olarak yönetmek için masaüstü uygulamasını veya CLI'ı kullanın, [kullanmadan önce becerileri denetleyin](https://skillshare.runkids.cc/docs/reference/commands/audit) ve [her aracın ne alacağını seçin](https://skillshare.runkids.cc/docs/how-to/daily-tasks/filtering-skills).

> Başka bir araçtan mı geliyorsunuz? [Geçiş Kılavuzu](https://skillshare.runkids.cc/docs/how-to/advanced/migration) · [Karşılaştırma](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

<a id="screenshots"></a>

## CLI ve Arayüz Ön İzlemesi

| Beceri Ayrıntısı | Güvenlik Denetimi |
|---|---|
| <img src=".github/assets/skill-detail-tui.png" alt="CLI eşitleme çıktısı" width="480" height="300"> | <img src=".github/assets/audit-tui.png" alt="Güvenlik denetimli CLI kurulumu" width="480" height="300"> |

| Web Kontrol Paneli | Web Arayüzü Becerileri |
|---|---|
| <img src=".github/assets/ui/web-dashboard-demo.png" alt="Web kontrol paneli genel görünümü" width="480"> | <img src=".github/assets/ui/web-skills-demo.png" alt="Web arayüzü beceriler sayfası" width="480"> |

## Kurulum

> [!TIP]
> **skillshare'i masaüstünüzden kullanın.** macOS (Apple Silicon), Windows ve Linux için [Skillshare App uygulamasını edinin](https://github.com/runkids/skillshare-app/releases/latest). İlk açılış CLI'ı kurmanıza veya bulmanıza, yapay zekâ araçlarınızı seçmenize ve ilk eşitlemenizi çalıştırmanıza yardımcı olur. [Kurulum kılavuzu](https://skillshare.runkids.cc/docs/getting-started/desktop-app).

<a id="desktop-app"></a>

### Masaüstü Uygulaması — görsel kurulum ve günlük yönetim

[Skillshare App](https://github.com/runkids/skillshare-app) becerileri, ajanları, MCP'yi ve kancaları bir masaüstü penceresinde toplar. Uygulamayı kurun, açın ve ilk açılış kurulumunu izleyin.

macOS (Apple Silicon), Homebrew ile:

```bash
brew tap runkids/tap
brew install --cask skillshare-app
```

**Windows / Linux veya manuel macOS kurulumu:** [En son uygulama yükleyicilerini indirin](https://github.com/runkids/skillshare-app/releases/latest). Platform ayrıntıları için [Masaüstü Uygulaması kılavuzuna](https://skillshare.runkids.cc/docs/getting-started/desktop-app) bakın.

### CLI: macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/runkids/skillshare/main/install.sh | sh
```

Betik varsayılan olarak `~/.local/bin` dizinine kurar; bu nedenle normal kurulumlar ve güncellemeler `sudo` gerektirmez. Yükleyici PATH kurulum talimatları yazdırırsa `skillshare` çalıştırmadan önce bunları uygulayın. Gelecekteki terminaller için önerilen satırı kabuk yapılandırmanıza (örneğin `~/.zshrc` veya `~/.bashrc`) ekleyin. Başka bir konum kullanmak için `INSTALL_DIR` ayarlayın.

### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/runkids/skillshare/main/install.ps1 | iex
```

### CLI: Homebrew

```bash
brew install skillshare
```

> **İpucu:** En son sürüme güncellemek için `skillshare upgrade` çalıştırın. Kurulum yönteminizi otomatik algılar ve gerisini halleder.

### CLI: Nix

```bash
nix profile add github:runkids/skillshare
```

`nix profile upgrade skillshare` ile güncelleyin. Flake'lerin etkin olduğu Nix 2.30+ gerektirir; daha eski Nix veya Lix sürümlerinde `nix profile install` kullanın. NixOS ve Home Manager yapılandırmaları flake'in `overlays.default` katmanını kullanabilir.

### CLI: PyPI

```bash
uv tool install skillshare-cli
```

Veya `pipx install skillshare-cli` ya da `pip install skillshare-cli`. skillshare'i kurmadan denemek için `uvx --from skillshare-cli skillshare` çalıştırın. `uv tool upgrade skillshare-cli` veya `pipx upgrade skillshare-cli` gibi kendi yükleyicinizle güncelleyin.

### GitHub Actions

```yaml
- uses: runkids/setup-skillshare@v1
  with:
    source: ./skills
- run: skillshare sync
```

Tüm seçenekler (denetim, proje modu, sürüm sabitleme) için [`setup-skillshare`](https://github.com/marketplace/actions/setup-skillshare) sayfasına bakın.

### Kısaltma (İsteğe Bağlı)

Kabuk yapılandırmanıza (`~/.zshrc` veya `~/.bashrc`) bir takma ad (alias) ekleyin:

```bash
alias ss='skillshare'
```

## Hızlı Başlangıç

```bash
skillshare init            # Yapılandırma, kaynak ve algılanan hedefleri oluştur
skillshare sync            # Becerileri tüm hedeflerle eşitle
```

## Nasıl Çalışır

- macOS / Linux: `~/.config/skillshare/`
- Windows: `%AppData%\skillshare\`

```
┌─────────────────────────────────────────────────────────────┐
│                    Kaynak Dizini (Source)                   │
│   ~/.config/skillshare/skills/    ← beceriler (SKILL.md)    │
│   ~/.config/skillshare/agents/    ← ajanlar                 │
│   ~/.config/skillshare/extras/    ← kurallar, komutlar vb.  │
└─────────────────────────────────────────────────────────────┘
                              │ sync
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
       ┌───────────┐   ┌───────────┐   ┌───────────┐
       │  Claude   │   │  OpenCode │   │ OpenClaw  │   ...
       └───────────┘   └───────────┘   └───────────┘
```

| Platform | Beceriler Kaynağı | Ajanlar Kaynağı | Ekstralar Kaynağı | Bağlantı Türü |
|----------|-------------------|-----------------|-------------------|---------------|
| macOS/Linux | `~/.config/skillshare/skills/` | `~/.config/skillshare/agents/` | `~/.config/skillshare/extras/` | Sembolik Bağlantılar (Symlinks) |
| Windows | `%AppData%\skillshare\skills\` | `%AppData%\skillshare\agents\` | `%AppData%\skillshare\extras\` | Klasörler için NTFS Junction (yönetici gerekmez); dosya sembolik bağlantıları Geliştirici Modu gerektirir, aksi hâlde kopyalanır |

| | Emir Kipinde (komut başına kurulum) | Bildirimsel (skillshare) |
|---|---|---|
| **Doğruluk kaynağı** | Beceriler bağımsız olarak kopyalanır | Tek kaynak → sembolik bağlantılar (veya kopyalar) |
| **Yeni makine kurulumu** | Her kurulumu manuel olarak yeniden çalıştır | `git clone` yapılandırması + `sync` |
| **Güvenlik denetimi** | Yok | Yerleşik `audit` + kurulum/güncellemede otomatik tarama |
| **Web kontrol paneli** | Yok | `skillshare ui` |
| **Çalışma zamanı bağımlılığı** | Node.js + npm | Yok (tek Go ikili dosyası) |

> [Tam karşılaştırma →](https://skillshare.runkids.cc/docs/understand/philosophy/comparison)

## Öne Çıkanlar

**Beceri kurma ve güncelleme** — GitHub, GitLab veya herhangi bir Git sunucusundan

```bash
skillshare install github.com/reponame/skills
skillshare update --all
skillshare target claude --mode copy  # sembolik bağlantılar çalışmıyorsa
```

**Sembolik bağlantı sorunları mı var?** — hedef başına kopyalama moduna geçin

```bash
skillshare target <name> --mode copy
skillshare sync
```

**Güvenlik denetimi** — beceriler ajanınıza ulaşmadan önce tarayın

```bash
skillshare audit
```

**Proje becerileri** — depo başına, kodunuzla birlikte commit edilir

```bash
skillshare init -p && skillshare sync
```

**Ajanlar** — özel ajanları ajan destekli hedeflerle eşitleyin

```bash
skillshare sync agents            # yalnızca ajanları eşitle
skillshare sync --all             # beceriler + ajanlar + ekstralar + MCP + kancaları birlikte eşitle
```

**Ekstralar** — kuralları, komutları, istemleri ve daha fazlasını yönetin

```bash
skillshare extras init rules          # "rules" adlı bir ekstra oluştur
skillshare sync --all                 # beceriler + ajanlar + ekstralar + MCP + kancaları birlikte eşitle
skillshare extras collect rules       # yerel dosyaları kaynağa geri topla
```

**MCP bağlantıları** — Claude Code, Codex, Pi, VS Code, OpenCode ve daha fazlası için bir kez yapılandırın

```bash
skillshare mcp add                    # rehberli URL veya JSON kurulumu
skillshare sync mcp --dry-run         # yerel yapılandırma değişikliklerini ön izle
skillshare sync mcp                   # bağlantı ayarlarını uygula
```

Tanımları `config.yaml` içinde tutun veya ayrı bir `mcp.yaml` dosyasına başvurun.
Örnekler, ortam değişkeni başvuruları ve mevcut bağlantıları içe aktarma için [MCP kurulumuna](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-mcp) bakın.

Yerel kancaları (hooks) çalıştırmadan yönetin:

```bash
skillshare hooks add check --file ./check.yaml
skillshare hooks sync --dry-run
skillshare hooks sync
```

**Eklentiler (Plugins)** — eksiksiz bir eklenti kurun ve hangi araçların alacağını seçin

```bash
skillshare plugin add                 # rehberli: kaynak, eklenti, hedefler, inceleme
skillshare plugin add owner/repo --target claude --target codex --no-tui
skillshare sync plugins --dry-run     # eklentiler sync --all komutundan ayrı eşitlenir
```

Mevcut yerel kurulumlar `plugin import` ile devralınabilir.
Bkz. [Araçlar arasında eklentileri yönetme](https://skillshare.runkids.cc/docs/how-to/daily-tasks/sharing-plugins).

**Kabuk tamamlama** — komutları, bayrakları ve alt komutları Tab ile tamamlayın

```bash
skillshare completion bash --install   # ayrıca: zsh, fish, powershell, nushell
```

**Yerel kontrol noktaları** — göndermeden (push) kaynak değişikliklerini commit edin

```bash
skillshare commit -m "Update review skill"
skillshare commit --dry-run
```

**Web kontrol paneli** — görsel kontrol paneli

```bash
skillshare ui
```

**Herdr paneli** — [Herdr](https://herdr.dev) içinde ajanlarınızın yanında skillshare: güncellemeler, eşitleme, denetlenmiş kurulumlar ve kaynak depo tek panelde

[Herdr panelini kurun →](https://skillshare.runkids.cc/docs/learn/with-herdr)

[Tüm komutlar ve kılavuzlar →](https://skillshare.runkids.cc/docs/reference/commands)

## Katkıda Bulunma

Katkılarınız memnuniyetle karşılanır! Önce bir issue açın, ardından testlerle birlikte bir taslak PR gönderin.
Kurulum ayrıntıları için [CONTRIBUTING.md](CONTRIBUTING.md) dosyasına bakın.

```bash
git clone https://github.com/runkids/skillshare.git && cd skillshare
make check  # biçimlendirme + lint + test
```

> [!TIP]
> Nereden başlayacağınızdan emin değil misiniz? [Açık issue'lara](https://github.com/runkids/skillshare/issues) göz atın veya kurulum gerektirmeyen geliştirme ortamı için [Playground](https://skillshare.runkids.cc/docs/learn/with-playground) deneyin.

## Katkıda Bulunanlar

[GitHub'da katkıda bulunanları görüntüleyin](https://github.com/runkids/skillshare/graphs/contributors)

### Topluluk teşekkürleri

Hata bildiren, iyileştirme öneren veya bir düzeltmeyi test etmeye yardımcı olan herkese teşekkürler.

<a href="https://github.com/leeeezx"><img src="https://github.com/leeeezx.png" width="50" style="border-radius:50%" alt="leeeezx"></a>
<a href="https://github.com/Vergil333"><img src="https://github.com/Vergil333.png" width="50" style="border-radius:50%" alt="Vergil333"></a>
<a href="https://github.com/romanr"><img src="https://github.com/romanr.png" width="50" style="border-radius:50%" alt="romanr"></a>
<a href="https://github.com/xocasdashdash"><img src="https://github.com/xocasdashdash.png" width="50" style="border-radius:50%" alt="xocasdashdash"></a>
<a href="https://github.com/philippe-granet"><img src="https://github.com/philippe-granet.png" width="50" style="border-radius:50%" alt="philippe-granet"></a>
<a href="https://github.com/terranc"><img src="https://github.com/terranc.png" width="50" style="border-radius:50%" alt="terranc"></a>
<a href="https://github.com/benrfairless"><img src="https://github.com/benrfairless.png" width="50" style="border-radius:50%" alt="benrfairless"></a>
<a href="https://github.com/nerveband"><img src="https://github.com/nerveband.png" width="50" style="border-radius:50%" alt="nerveband"></a>
<a href="https://github.com/EarthChen"><img src="https://github.com/EarthChen.png" width="50" style="border-radius:50%" alt="EarthChen"></a>
<a href="https://github.com/gdm257"><img src="https://github.com/gdm257.png" width="50" style="border-radius:50%" alt="gdm257"></a>
<a href="https://github.com/skovtunenko"><img src="https://github.com/skovtunenko.png" width="50" style="border-radius:50%" alt="skovtunenko"></a>
<a href="https://github.com/TyceHerrman"><img src="https://github.com/TyceHerrman.png" width="50" style="border-radius:50%" alt="TyceHerrman"></a>
<a href="https://github.com/1am2syman"><img src="https://github.com/1am2syman.png" width="50" style="border-radius:50%" alt="1am2syman"></a>
<a href="https://github.com/thealokkr"><img src="https://github.com/thealokkr.png" width="50" style="border-radius:50%" alt="thealokkr"></a>
<a href="https://github.com/JasonLandbridge"><img src="https://github.com/JasonLandbridge.png" width="50" style="border-radius:50%" alt="JasonLandbridge"></a>
<a href="https://github.com/masonc15"><img src="https://github.com/masonc15.png" width="50" style="border-radius:50%" alt="masonc15"></a>
<a href="https://github.com/richardwhatever"><img src="https://github.com/richardwhatever.png" width="50" style="border-radius:50%" alt="richardwhatever"></a>
<a href="https://github.com/reneleonhardt"><img src="https://github.com/reneleonhardt.png" width="50" style="border-radius:50%" alt="reneleonhardt"></a>
<a href="https://github.com/ndeybach"><img src="https://github.com/ndeybach.png" width="50" style="border-radius:50%" alt="ndeybach"></a>
<a href="https://github.com/hhh2210"><img src="https://github.com/hhh2210.png" width="50" style="border-radius:50%" alt="hhh2210"></a>
<a href="https://github.com/leoarry"><img src="https://github.com/leoarry.png" width="50" style="border-radius:50%" alt="leoarry"></a>
<a href="https://github.com/salmonumbrella"><img src="https://github.com/salmonumbrella.png" width="50" style="border-radius:50%" alt="salmonumbrella"></a>
<a href="https://github.com/daylamtayari"><img src="https://github.com/daylamtayari.png" width="50" style="border-radius:50%" alt="daylamtayari"></a>
<a href="https://github.com/dstotijn"><img src="https://github.com/dstotijn.png" width="50" style="border-radius:50%" alt="dstotijn"></a>
<a href="https://github.com/ipruning"><img src="https://github.com/ipruning.png" width="50" style="border-radius:50%" alt="ipruning"></a>
<a href="https://github.com/massukio"><img src="https://github.com/massukio.png" width="50" style="border-radius:50%" alt="massukio"></a>
<a href="https://github.com/nathanvale"><img src="https://github.com/nathanvale.png" width="50" style="border-radius:50%" alt="nathanvale"></a>
<a href="https://github.com/LloydPowell771"><img src="https://github.com/LloydPowell771.png" width="50" style="border-radius:50%" alt="LloydPowell771"></a>
<a href="https://github.com/kevincobain2000"><img src="https://github.com/kevincobain2000.png" width="50" style="border-radius:50%" alt="kevincobain2000"></a>
<a href="https://github.com/StephenPAdams"><img src="https://github.com/StephenPAdams.png" width="50" style="border-radius:50%" alt="StephenPAdams"></a>
<a href="https://github.com/mk-imagine"><img src="https://github.com/mk-imagine.png" width="50" style="border-radius:50%" alt="mk-imagine"></a>
<a href="https://github.com/Curtion"><img src="https://github.com/Curtion.png" width="50" style="border-radius:50%" alt="Curtion"></a>
<a href="https://github.com/amdoi7"><img src="https://github.com/amdoi7.png" width="50" style="border-radius:50%" alt="amdoi7"></a>
<a href="https://github.com/jessica-engel"><img src="https://github.com/jessica-engel.png" width="50" style="border-radius:50%" alt="jessica-engel"></a>
<a href="https://github.com/AlimuratYusup"><img src="https://github.com/AlimuratYusup.png" width="50" style="border-radius:50%" alt="AlimuratYusup"></a>
<a href="https://github.com/thor-shuang"><img src="https://github.com/thor-shuang.png" width="50" style="border-radius:50%" alt="thor-shuang"></a>
<a href="https://github.com/bishopmatthew"><img src="https://github.com/bishopmatthew.png" width="50" style="border-radius:50%" alt="bishopmatthew"></a>
<a href="https://github.com/chaosky"><img src="https://github.com/chaosky.png" width="50" style="border-radius:50%" alt="chaosky"></a>
<a href="https://github.com/iFwu"><img src="https://github.com/iFwu.png" width="50" style="border-radius:50%" alt="iFwu"></a>
<a href="https://github.com/ildunari"><img src="https://github.com/ildunari.png" width="50" style="border-radius:50%" alt="ildunari"></a>
<a href="https://github.com/vieGPT"><img src="https://github.com/vieGPT.png" width="50" style="border-radius:50%" alt="vieGPT"></a>
<a href="https://github.com/aestilog"><img src="https://github.com/aestilog.png" width="50" style="border-radius:50%" alt="aestilog"></a>
<a href="https://github.com/xarthurx"><img src="https://github.com/xarthurx.png" width="50" style="border-radius:50%" alt="xarthurx"></a>
<a href="https://github.com/m0cun"><img src="https://github.com/m0cun.png" width="50" style="border-radius:50%" alt="m0cun"></a>
<a href="https://github.com/bit3125"><img src="https://github.com/bit3125.png" width="50" style="border-radius:50%" alt="bit3125"></a>
<a href="https://github.com/eekryuos"><img src="https://github.com/eekryuos.png" width="50" style="border-radius:50%" alt="eekryuos"></a>
<a href="https://github.com/Bongseop-Kim"><img src="https://github.com/Bongseop-Kim.png" width="50" style="border-radius:50%" alt="Bongseop-Kim"></a>
<a href="https://github.com/sophodex"><img src="https://github.com/sophodex.png" width="50" style="border-radius:50%" alt="sophodex"></a>
<a href="https://github.com/PeterTianbuhan"><img src="https://github.com/PeterTianbuhan.png" width="50" style="border-radius:50%" alt="PeterTianbuhan"></a>
<a href="https://github.com/dotned"><img src="https://github.com/dotned.png" width="50" style="border-radius:50%" alt="dotned"></a>
<a href="https://github.com/ismferd"><img src="https://github.com/ismferd.png" width="50" style="border-radius:50%" alt="ismferd"></a>
<a href="https://github.com/jblackburn21"><img src="https://github.com/jblackburn21.png" width="50" style="border-radius:50%" alt="jblackburn21"></a>
<a href="https://github.com/jnhu76"><img src="https://github.com/jnhu76.png" width="50" style="border-radius:50%" alt="jnhu76"></a>
<a href="https://github.com/jacobleft"><img src="https://github.com/jacobleft.png" width="50" style="border-radius:50%" alt="jacobleft"></a>
<a href="https://github.com/rhysmcneill"><img src="https://github.com/rhysmcneill.png" width="50" style="border-radius:50%" alt="rhysmcneill"></a>
<a href="https://github.com/druellan"><img src="https://github.com/druellan.png" width="50" style="border-radius:50%" alt="druellan"></a>
<a href="https://github.com/12britz"><img src="https://github.com/12britz.png" width="50" style="border-radius:50%" alt="12britz"></a>
<a href="https://github.com/askpatrickw"><img src="https://github.com/askpatrickw.png" width="50" style="border-radius:50%" alt="askpatrickw"></a>
<a href="https://github.com/Almost42"><img src="https://github.com/Almost42.png" width="50" style="border-radius:50%" alt="Almost42"></a>
<a href="https://github.com/Brett-Best"><img src="https://github.com/Brett-Best.png" width="50" style="border-radius:50%" alt="Brett-Best"></a>
<a href="https://github.com/isCopyman"><img src="https://github.com/isCopyman.png" width="50" style="border-radius:50%" alt="isCopyman"></a>
<a href="https://github.com/svob"><img src="https://github.com/svob.png" width="50" style="border-radius:50%" alt="svob"></a>
<a href="https://github.com/michal-grzelak"><img src="https://github.com/michal-grzelak.png" width="50" style="border-radius:50%" alt="michal-grzelak"></a>
<a href="https://github.com/shikbupt"><img src="https://github.com/shikbupt.png" width="50" style="border-radius:50%" alt="shikbupt"></a>
<a href="https://github.com/LeoYeAI"><img src="https://github.com/LeoYeAI.png" width="50" style="border-radius:50%" alt="LeoYeAI"></a>
<a href="https://github.com/TIR44"><img src="https://github.com/TIR44.png" width="50" style="border-radius:50%" alt="TIR44"></a>
<a href="https://github.com/Ajaymamtora"><img src="https://github.com/Ajaymamtora.png" width="50" style="border-radius:50%" alt="Ajaymamtora"></a>
<a href="https://github.com/vishaldialpad"><img src="https://github.com/vishaldialpad.png" width="50" style="border-radius:50%" alt="vishaldialpad"></a>
<a href="https://github.com/kankan0829"><img src="https://github.com/kankan0829.png" width="50" style="border-radius:50%" alt="kankan0829"></a>
<a href="https://github.com/dnabb"><img src="https://github.com/dnabb.png" width="50" style="border-radius:50%" alt="dnabb"></a>
<a href="https://github.com/thinhngotony"><img src="https://github.com/thinhngotony.png" width="50" style="border-radius:50%" alt="thinhngotony"></a>
<a href="https://github.com/skaurus"><img src="https://github.com/skaurus.png" width="50" style="border-radius:50%" alt="skaurus"></a>
<a href="https://github.com/jamesbraza"><img src="https://github.com/jamesbraza.png" width="50" style="border-radius:50%" alt="jamesbraza"></a>
<a href="https://github.com/2BAB"><img src="https://github.com/2BAB.png" width="50" style="border-radius:50%" alt="2BAB"></a>
<a href="https://github.com/zdlldz"><img src="https://github.com/zdlldz.png" width="50" style="border-radius:50%" alt="zdlldz"></a>
<a href="https://github.com/elstiaan"><img src="https://github.com/elstiaan.png" width="50" style="border-radius:50%" alt="elstiaan"></a>
<a href="https://github.com/FaintFlower"><img src="https://github.com/FaintFlower.png" width="50" style="border-radius:50%" alt="FaintFlower"></a>
<a href="https://github.com/yantinglin21"><img src="https://github.com/yantinglin21.png" width="50" style="border-radius:50%" alt="yantinglin21"></a>
<a href="https://github.com/chung1912"><img src="https://github.com/chung1912.png" width="50" style="border-radius:50%" alt="chung1912"></a>
<a href="https://github.com/r-fynn"><img src="https://github.com/r-fynn.png" width="50" style="border-radius:50%" alt="r-fynn"></a>
<a href="https://github.com/cescox"><img src="https://github.com/cescox.png" width="50" style="border-radius:50%" alt="cescox"></a>
<a href="https://github.com/hhdebb"><img src="https://github.com/hhdebb.png" width="50" style="border-radius:50%" alt="hhdebb"></a>
<a href="https://github.com/Snurppa"><img src="https://github.com/Snurppa.png" width="50" style="border-radius:50%" alt="Snurppa"></a>
<a href="https://github.com/s0undt3ch"><img src="https://github.com/s0undt3ch.png" width="50" style="border-radius:50%" alt="s0undt3ch"></a>
<a href="https://github.com/danscheer"><img src="https://github.com/danscheer.png" width="50" style="border-radius:50%" alt="danscheer"></a>
<a href="https://github.com/7zq12lvm-b"><img src="https://github.com/7zq12lvm-b.png" width="50" style="border-radius:50%" alt="7zq12lvm-b"></a>
<a href="https://github.com/DarkiT"><img src="https://github.com/DarkiT.png" width="50" style="border-radius:50%" alt="DarkiT"></a>
<a href="https://github.com/harisonw"><img src="https://github.com/harisonw.png" width="50" style="border-radius:50%" alt="harisonw"></a>
<a href="https://github.com/wuhaoyujerry"><img src="https://github.com/wuhaoyujerry.png" width="50" style="border-radius:50%" alt="wuhaoyujerry"></a>
<a href="https://github.com/star-nebula"><img src="https://github.com/star-nebula.png" width="50" style="border-radius:50%" alt="star-nebula"></a>
<a href="https://github.com/AdamMagued"><img src="https://github.com/AdamMagued.png" width="50" style="border-radius:50%" alt="AdamMagued"></a>
<a href="https://github.com/mbury"><img src="https://github.com/mbury.png" width="50" style="border-radius:50%" alt="mbury"></a>
<a href="https://github.com/483218131"><img src="https://github.com/483218131.png" width="50" style="border-radius:50%" alt="483218131"></a>
<a href="https://github.com/neilforest7"><img src="https://github.com/neilforest7.png" width="50" style="border-radius:50%" alt="neilforest7"></a>
<a href="https://github.com/johnkattenhorn"><img src="https://github.com/johnkattenhorn.png" width="50" style="border-radius:50%" alt="johnkattenhorn"></a>
<a href="https://github.com/aleps001"><img src="https://github.com/aleps001.png" width="50" style="border-radius:50%" alt="aleps001"></a>
<a href="https://github.com/jskswamy"><img src="https://github.com/jskswamy.png" width="50" style="border-radius:50%" alt="jskswamy"></a>
<a href="https://github.com/heyAidy"><img src="https://github.com/heyAidy.png" width="50" style="border-radius:50%" alt="heyAidy"></a>
<a href="https://github.com/trojanwk"><img src="https://github.com/trojanwk.png" width="50" style="border-radius:50%" alt="trojanwk"></a>
<a href="https://github.com/charitarthchugh"><img src="https://github.com/charitarthchugh.png" width="50" style="border-radius:50%" alt="charitarthchugh"></a>
<a href="https://github.com/servaasvdc"><img src="https://github.com/servaasvdc.png" width="50" style="border-radius:50%" alt="servaasvdc"></a>
<a href="https://github.com/tomerzcod7"><img src="https://github.com/tomerzcod7.png" width="50" style="border-radius:50%" alt="tomerzcod7"></a>
<a href="https://github.com/ErykJakubik"><img src="https://github.com/ErykJakubik.png" width="50" style="border-radius:50%" alt="ErykJakubik"></a>
<a href="https://github.com/maciejniec"><img src="https://github.com/maciejniec.png" width="50" style="border-radius:50%" alt="maciejniec"></a>
<a href="https://github.com/jani-nieminen-framehaus"><img src="https://github.com/jani-nieminen-framehaus.png" width="50" style="border-radius:50%" alt="jani-nieminen-framehaus"></a>
<a href="https://github.com/endgame302"><img src="https://github.com/endgame302.png" width="50" style="border-radius:50%" alt="endgame302"></a>
<a href="https://github.com/eirikc"><img src="https://github.com/eirikc.png" width="50" style="border-radius:50%" alt="eirikc"></a>
<a href="https://github.com/AlexisSerneels"><img src="https://github.com/AlexisSerneels.png" width="50" style="border-radius:50%" alt="AlexisSerneels"></a>
<a href="https://github.com/digitalflex-io"><img src="https://github.com/digitalflex-io.png" width="50" style="border-radius:50%" alt="digitalflex-io"></a>
<a href="https://github.com/WslzGmzs"><img src="https://github.com/WslzGmzs.png" width="50" style="border-radius:50%" alt="WslzGmzs"></a>
<a href="https://github.com/NieZhuZhu"><img src="https://github.com/NieZhuZhu.png" width="50" style="border-radius:50%" alt="NieZhuZhu"></a>
<a href="https://github.com/vivekkushalch"><img src="https://github.com/vivekkushalch.png" width="50" style="border-radius:50%" alt="vivekkushalch"></a>
<a href="https://github.com/raphaelbahat"><img src="https://github.com/raphaelbahat.png" width="50" style="border-radius:50%" alt="raphaelbahat"></a>
<a href="https://github.com/dev-seahouse"><img src="https://github.com/dev-seahouse.png" width="50" style="border-radius:50%" alt="dev-seahouse"></a>
<a href="https://github.com/cazador481"><img src="https://github.com/cazador481.png" width="50" style="border-radius:50%" alt="cazador481"></a>
<a href="https://github.com/SaladDay"><img src="https://github.com/SaladDay.png" width="50" style="border-radius:50%" alt="SaladDay"></a>

---

## skillshare'e Destek Olun ❤️

skillshare ücretsiz ve açık kaynaklıdır.

skillshare size zaman kazandırıyorsa bana bir kahve ısmarlamayı düşünebilirsiniz.
Her katkı sürekli geliştirmeye, platformlar arası testlere,
dokümantasyona ve bakıma yardımcı olur.

[☕ Bana bir kahve ısmarlayın](https://ko-fi.com/williehung)

GitHub'da skillshare'e ⭐ vererek de yardımcı olabilirsiniz!

## Yıldız Geçmişi

<a href="https://star-history.dera.page/runkids/skillshare">
 <picture>
   <source media="(prefers-color-scheme: dark)" srcset="https://star-history.dera.page/svg?repos=runkids/skillshare&theme=dark" />
   <source media="(prefers-color-scheme: light)" srcset="https://star-history.dera.page/svg?repos=runkids/skillshare" />
   <img alt="Star History Chart" src="https://star-history.dera.page/svg?repos=runkids/skillshare" />
 </picture>
</a>

---

## Lisans

MIT
