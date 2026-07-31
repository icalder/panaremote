/**
 * Panaremote Service Worker
 *
 * Caching strategy:
 *   - HTML documents:    Network First (always check for updates, fall back to cache when offline)
 *   - Static assets:     Cache First  (icons, manifest — serve instantly, update via cache version)
 *   - API requests:       Network Only (never cache TV control commands)
 */

const CACHE_NAME = "panaremote-v1";
const ASSETS_TO_CACHE = [
    "/index.html",
    "/manifest.json",
    "/sw.js",
    "/icon-48.png",
    "/icon-96.png",
    "/icon-144.png",
    "/icon-152.png",
    "/icon-168.png",
    "/icon-192.png",
    "/icon-512.png"
];

/**
 * Returns true if the request is for an HTML document.
 */
function isHTMLRequest(request) {
    return request.mode === "navigate" ||
        request.headers.get("accept")?.includes("text/html");
}

/**
 * Returns true if the request is for an API endpoint.
 */
function isAPIRequest(request) {
    const url = new URL(request.url);
    return url.pathname.startsWith("/api/");
}

// Install: cache all core static assets so the app works offline
self.addEventListener("install", (event) => {
    event.waitUntil(
        caches.open(CACHE_NAME).then((cache) => {
            return cache.addAll(ASSETS_TO_CACHE);
        })
    );
    // Force the waiting service worker to become active immediately
    self.skipWaiting();
});

// Activate: remove any old caches that are no longer needed
self.addEventListener("activate", (event) => {
    event.waitUntil(
        caches.keys().then((cacheNames) => {
            return Promise.all(
                cacheNames.map((name) => {
                    if (name !== CACHE_NAME) {
                        return caches.delete(name);
                    }
                })
            );
        })
    );
    self.clients.claim();
});

// Fetch: apply different strategies based on request type
self.addEventListener("fetch", (event) => {
    const request = event.request;

    // Never cache API requests — always hit the network
    if (isAPIRequest(request)) {
        return;
    }

    // Only handle GET requests
    if (request.method !== "GET") {
        return;
    }

    if (isHTMLRequest(request)) {
        // Network First for HTML: always check for updates
        event.respondWith(
            fetch(request)
                .then((networkResponse) => {
                    // Cache the latest HTML for offline use
                    const cloned = networkResponse.clone();
                    caches.open(CACHE_NAME).then((cache) => {
                        cache.put(request, cloned);
                    });
                    return networkResponse;
                })
                .catch(() => {
                    // Offline: serve cached HTML
                    return caches.match(request);
                })
        );
    } else {
        // Cache First for static assets: serve instantly, fall back to network
        event.respondWith(
            caches.match(request).then((cachedResponse) => {
                if (cachedResponse) {
                    // Update cache in the background for next time
                    fetch(request).then((networkResponse) => {
                        caches.open(CACHE_NAME).then((cache) => {
                            cache.put(request, networkResponse);
                        });
                    }).catch(() => {});
                    return cachedResponse;
                }

                // Not cached: fetch from network and cache for future use
                return fetch(request).then((networkResponse) => {
                    if (!networkResponse || networkResponse.status !== 200) {
                        return networkResponse;
                    }

                    return caches.open(CACHE_NAME).then((cache) => {
                        cache.put(request, networkResponse.clone());
                        return networkResponse;
                    });
                }).catch(() => {
                    // Both cache and network failed — we're offline
                    return new Response("Offline", {
                        status: 503,
                        statusText: "Service Unavailable"
                    });
                });
            })
        );
    }
});
