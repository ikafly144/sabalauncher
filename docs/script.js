// Mobile Navigation Toggle
document.addEventListener('DOMContentLoaded', function () {
    const navToggle = document.querySelector('.nav-toggle');
    const navMenu = document.querySelector('.nav-menu');

    if (navToggle && navMenu) {
        navToggle.addEventListener('click', function () {
            navMenu.classList.toggle('nav-menu-active');
            navToggle.classList.toggle('nav-toggle-active');
        });

        // Close menu when clicking on a link
        const navLinks = document.querySelectorAll('.nav-link');
        navLinks.forEach(link => {
            link.addEventListener('click', () => {
                navMenu.classList.remove('nav-menu-active');
                navToggle.classList.remove('nav-toggle-active');
            });
        });
    }
});

// Smooth Scrolling for Anchor Links
document.addEventListener('DOMContentLoaded', function () {
    const links = document.querySelectorAll('a[href^="#"]');

    links.forEach(link => {
        link.addEventListener('click', function (e) {
            e.preventDefault();

            const targetId = this.getAttribute('href');
            const targetSection = document.querySelector(targetId);

            if (targetSection) {
                const headerHeight = document.querySelector('.header').offsetHeight;
                const targetPosition = targetSection.offsetTop - headerHeight - 20;

                window.scrollTo({
                    top: targetPosition,
                    behavior: 'smooth'
                });
            }
        });
    });
});

// Header Background Change on Scroll
document.addEventListener('DOMContentLoaded', function () {
    const header = document.querySelector('.header');

    function updateHeader() {
        if (window.scrollY > 100) {
            header.style.background = 'rgba(255, 255, 255, 0.98)';
            header.style.boxShadow = '0 2px 20px rgba(0, 0, 0, 0.1)';
        } else {
            header.style.background = 'rgba(255, 255, 255, 0.95)';
            header.style.boxShadow = 'none';
        }
    }

    window.addEventListener('scroll', updateHeader);
    updateHeader(); // Initial call
});

// Intersection Observer for Animation
document.addEventListener('DOMContentLoaded', function () {
    const observerOptions = {
        threshold: 0.1,
        rootMargin: '0px 0px -50px 0px'
    };

    const observer = new IntersectionObserver(function (entries) {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                entry.target.style.opacity = '1';
                entry.target.style.transform = 'translateY(0)';
            }
        });
    }, observerOptions);

    // Observe elements that should animate in
    const animatedElements = document.querySelectorAll('.feature-card, .download-card, .doc-card');
    animatedElements.forEach(el => {
        el.style.opacity = '0';
        el.style.transform = 'translateY(30px)';
        el.style.transition = 'opacity 0.6s ease, transform 0.6s ease';
        observer.observe(el);
    });
});

// Dynamic Version Loading
document.addEventListener('DOMContentLoaded', function () {
    async function loadLatestVersion() {
        try {
            const response = await fetch('https://api.github.com/repos/ikafly144/sabalauncher/releases/latest');
            if (response.ok) {
                const data = await response.json();
                const version = data.tag_name;
                const downloadUrl = data.assets.find(asset => asset.name.endsWith('.msi'))?.browser_download_url;

                // Update version display
                const versionElements = document.querySelectorAll('.version-display');
                versionElements.forEach(el => {
                    el.textContent = version;
                });

                // Update download links
                const downloadLinks = document.querySelectorAll('a[href*="releases/latest"]');
                if (downloadUrl) {
                    downloadLinks.forEach(link => {
                        link.href = downloadUrl;
                    });
                }
            }
        } catch (error) {
            console.log('Could not load latest version:', error);
        }
    }

    loadLatestVersion();
});

// Copy Code Block Functionality
document.addEventListener('DOMContentLoaded', function () {
    const codeBlocks = document.querySelectorAll('.code-block');

    codeBlocks.forEach(block => {
        // Add copy button
        const copyButton = document.createElement('button');
        copyButton.textContent = 'コピー';
        copyButton.className = 'copy-button';
        copyButton.style.cssText = `
            position: absolute;
            top: 10px;
            right: 10px;
            background: rgba(255, 255, 255, 0.2);
            border: 1px solid rgba(255, 255, 255, 0.3);
            color: white;
            padding: 5px 10px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 12px;
            transition: all 0.3s ease;
        `;

        block.style.position = 'relative';
        block.appendChild(copyButton);

        copyButton.addEventListener('click', async function () {
            const code = block.querySelector('code');
            if (code) {
                try {
                    await navigator.clipboard.writeText(code.textContent);
                    copyButton.textContent = 'コピー済み!';
                    copyButton.style.background = 'rgba(16, 185, 129, 0.8)';

                    setTimeout(() => {
                        copyButton.textContent = 'コピー';
                        copyButton.style.background = 'rgba(255, 255, 255, 0.2)';
                    }, 2000);
                } catch (err) {
                    console.error('Failed to copy text: ', err);
                }
            }
        });

        copyButton.addEventListener('mouseenter', function () {
            this.style.background = 'rgba(255, 255, 255, 0.3)';
        });

        copyButton.addEventListener('mouseleave', function () {
            if (this.textContent === 'コピー') {
                this.style.background = 'rgba(255, 255, 255, 0.2)';
            }
        });
    });
});

// Loading Animation for External Links
document.addEventListener('DOMContentLoaded', function () {
    const externalLinks = document.querySelectorAll('a[target="_blank"]');

    externalLinks.forEach(link => {
        link.addEventListener('click', function () {
            const originalText = this.innerHTML;
            this.innerHTML = '<span class="loading">読み込み中...</span>';

            setTimeout(() => {
                this.innerHTML = originalText;
            }, 1000);
        });
    });
});

// Parallax Effect for Hero Section
document.addEventListener('DOMContentLoaded', function () {
    const heroSection = document.querySelector('.hero');

    if (heroSection) {
        window.addEventListener('scroll', function () {
            const scrolled = window.pageYOffset;
            const rate = scrolled * -0.5;

            if (scrolled < heroSection.offsetHeight) {
                heroSection.style.transform = `translateY(${rate}px)`;
            }
        });
    }
});

// Enhanced Button Interactions
document.addEventListener('DOMContentLoaded', function () {
    const buttons = document.querySelectorAll('.btn');

    buttons.forEach(button => {
        button.addEventListener('mouseenter', function () {
            this.style.transform = 'translateY(-2px) scale(1.02)';
        });

        button.addEventListener('mouseleave', function () {
            this.style.transform = 'translateY(0) scale(1)';
        });

        button.addEventListener('mousedown', function () {
            this.style.transform = 'translateY(0) scale(0.98)';
        });

        button.addEventListener('mouseup', function () {
            this.style.transform = 'translateY(-2px) scale(1.02)';
        });
    });
});

// Feature Card Hover Effects
document.addEventListener('DOMContentLoaded', function () {
    const featureCards = document.querySelectorAll('.feature-card');

    featureCards.forEach(card => {
        card.addEventListener('mouseenter', function () {
            const icon = this.querySelector('.feature-icon');
            if (icon) {
                icon.style.transform = 'scale(1.1) rotate(5deg)';
            }
        });

        card.addEventListener('mouseleave', function () {
            const icon = this.querySelector('.feature-icon');
            if (icon) {
                icon.style.transform = 'scale(1) rotate(0deg)';
            }
        });
    });
});

// Status Indicator (Online/Offline)
document.addEventListener('DOMContentLoaded', function () {
    function updateOnlineStatus() {
        const statusIndicator = document.querySelector('.status-indicator');
        if (statusIndicator) {
            if (navigator.onLine) {
                statusIndicator.textContent = 'オンライン';
                statusIndicator.className = 'status-indicator online';
            } else {
                statusIndicator.textContent = 'オフライン';
                statusIndicator.className = 'status-indicator offline';
            }
        }
    }

    window.addEventListener('online', updateOnlineStatus);
    window.addEventListener('offline', updateOnlineStatus);
    updateOnlineStatus();
});

// Theme Toggle (if implemented in future)
document.addEventListener('DOMContentLoaded', function () {
    const themeToggle = document.querySelector('.theme-toggle');

    if (themeToggle) {
        themeToggle.addEventListener('click', function () {
            document.body.classList.toggle('dark-theme');
            localStorage.setItem('theme', document.body.classList.contains('dark-theme') ? 'dark' : 'light');
        });

        // Load saved theme
        const savedTheme = localStorage.getItem('theme');
        if (savedTheme === 'dark') {
            document.body.classList.add('dark-theme');
        }
    }
});

// Performance Optimization: Lazy Loading for Images
document.addEventListener('DOMContentLoaded', function () {
    const images = document.querySelectorAll('img[data-src]');

    const imageObserver = new IntersectionObserver((entries, observer) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                const img = entry.target;
                img.src = img.dataset.src;
                img.classList.remove('lazy');
                imageObserver.unobserve(img);
            }
        });
    });

    images.forEach(img => imageObserver.observe(img));
});

// Analytics (placeholder for future implementation)
function trackEvent(category, action, label) {
    if (typeof gtag !== 'undefined' && typeof category === 'string' && typeof action === 'string') {
        gtag('event', action, {
            event_category: category,
            event_label: String(label || '')
        });
    }
}

// Track download clicks
document.addEventListener('DOMContentLoaded', function () {
    const downloadLinks = document.querySelectorAll('a[href*="releases"], a[href*="download"]');

    downloadLinks.forEach(link => {
        link.addEventListener('click', function () {
            trackEvent('Download', 'click', this.href);
        });
    });
});

console.log('SabaLauncher website loaded successfully! 🚀');
