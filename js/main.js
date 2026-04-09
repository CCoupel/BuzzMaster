/* ============================================
   BuzzMaster - Marketing Site Scripts
   ============================================ */

/* ------------------------------------------------------------------
   GitHub Releases — dynamic version + download links
   Fetches the latest release from the GitHub API and updates:
     - #hero-version       : tag name (e.g. "v3.3.3")
     - #hero-version-date  : publish date in French locale
     - #dl-windows         : direct .exe download link
     - #dl-linux           : direct arm64 binary download link
   Falls back silently to the static values already in the HTML if
   the fetch fails (network error, rate-limit, etc.).
   ------------------------------------------------------------------ */
(function fetchLatestRelease() {
    const API_URL = 'https://api.github.com/repos/CCoupel/BuzzMaster/releases/latest';

    fetch(API_URL, { headers: { Accept: 'application/vnd.github+json' } })
        .then(function(res) {
            if (!res.ok) throw new Error('GitHub API responded with ' + res.status);
            return res.json();
        })
        .then(function(release) {
            /* --- Version badge --- */
            var tagName = release.tag_name || '';
            var versionEl = document.getElementById('hero-version');
            var dateEl    = document.getElementById('hero-version-date');

            if (versionEl && tagName) {
                versionEl.textContent = tagName;
            }

            if (dateEl && release.published_at) {
                var d = new Date(release.published_at);
                var formatted = d.toLocaleDateString('fr-FR', {
                    day:   'numeric',
                    month: 'long',
                    year:  'numeric'
                });
                dateEl.textContent = formatted;
            }

            /* --- Download links --- */
            var assets  = Array.isArray(release.assets) ? release.assets : [];
            var version = tagName.replace(/^v/, ''); // "3.3.3"

            var windowsAsset = assets.find(function(a) {
                return /windows.*amd64.*\.exe$/i.test(a.name) ||
                       /amd64.*windows.*\.exe$/i.test(a.name);
            });
            var linuxAsset = assets.find(function(a) {
                return /linux.*arm64/i.test(a.name) && !/\.exe$/i.test(a.name);
            });

            /* Fallback: build conventional URL from version tag */
            var releasePage = release.html_url || 'https://github.com/CCoupel/BuzzMaster/releases/latest';
            var baseUrl = 'https://github.com/CCoupel/BuzzMaster/releases/download/' + tagName;

            var windowsHref = windowsAsset
                ? windowsAsset.browser_download_url
                : (version ? baseUrl + '/buzzcontrol-v' + version + '-windows-amd64.exe' : releasePage);
            var linuxHref = linuxAsset
                ? linuxAsset.browser_download_url
                : (version ? baseUrl + '/buzzcontrol-v' + version + '-linux-arm64' : releasePage);

            var dlWindows = document.getElementById('dl-windows');
            var dlLinux   = document.getElementById('dl-linux');

            if (dlWindows) dlWindows.href = windowsHref;
            if (dlLinux)   dlLinux.href   = linuxHref;
        })
        .catch(function(err) {
            /* Silent fallback — static HTML values remain */
            console.warn('[BuzzMaster] Could not fetch latest release:', err.message);
        });
})();

// Header scroll effect
document.addEventListener('DOMContentLoaded', () => {
    const header = document.querySelector('.header');

    // Add shadow on scroll
    window.addEventListener('scroll', () => {
        if (window.scrollY > 50) {
            header.style.boxShadow = '0 4px 20px rgba(0, 0, 0, 0.3)';
        } else {
            header.style.boxShadow = 'none';
        }
    });

    // Smooth scroll for anchor links
    document.querySelectorAll('a[href^="#"]').forEach(anchor => {
        anchor.addEventListener('click', function (e) {
            e.preventDefault();
            const target = document.querySelector(this.getAttribute('href'));
            if (target) {
                target.scrollIntoView({
                    behavior: 'smooth',
                    block: 'start'
                });
            }
        });
    });

    // Animate elements on scroll
    const observerOptions = {
        threshold: 0.1,
        rootMargin: '0px 0px -50px 0px'
    };

    const observer = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                entry.target.style.opacity = '1';
                entry.target.style.transform = 'translateY(0)';
            }
        });
    }, observerOptions);

    // Observe feature cards and component cards
    document.querySelectorAll('.feature-card, .component-card, .download-card').forEach(card => {
        card.style.opacity = '0';
        card.style.transform = 'translateY(20px)';
        card.style.transition = 'opacity 0.5s ease, transform 0.5s ease';
        observer.observe(card);
    });

    // Slideshow functionality
    document.querySelectorAll('.slideshow').forEach(slideshow => {
        const slides = slideshow.querySelectorAll('.slide');
        const interval = parseInt(slideshow.dataset.interval) || 3000;
        let currentIndex = 0;

        if (slides.length <= 1) return;

        setInterval(() => {
            slides[currentIndex].classList.remove('active');
            currentIndex = (currentIndex + 1) % slides.length;
            slides[currentIndex].classList.add('active');
        }, interval);
    });
});
