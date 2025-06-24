# GitHub Pages for SabaLauncher

This directory contains the static website for SabaLauncher hosted on GitHub Pages.

## Files

- `index.html` - Main landing page
- `docs.html` - Documentation page
- `404.html` - Custom 404 error page
- `styles.css` - Main stylesheet
- `script.js` - JavaScript functionality
- `assets/` - Images and static assets

## Development

To test locally:

```bash
# Navigate to docs directory
cd docs

# Start a simple HTTP server
python -m http.server 8000
# or
npx serve .

# Open http://localhost:8000 in your browser
```

## Deployment

The site is automatically deployed to GitHub Pages via GitHub Actions when changes are pushed to the main branch.

## Features

- Responsive design
- Modern UI with animations
- SEO optimized
- Mobile-friendly navigation
- Automatic version detection from GitHub releases
- Copy-to-clipboard functionality for code blocks
- Smooth scrolling navigation
- Interactive elements with hover effects

## License

MIT License - see [LICENSE](../LICENSE) for details.
