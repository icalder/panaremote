self.addEventListener('fetch', (e) => {
    // Basic pass-through service worker for PWA criteria
    e.respondWith(fetch(e.request).catch(() => caches.match(e.request)));
});