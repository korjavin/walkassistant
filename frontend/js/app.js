// Walk Assistant JavaScript

document.addEventListener('DOMContentLoaded', function() {
    // Initialize map
    const map = L.map('map-container').setView([0, 0], 13);

    // Add OpenStreetMap tiles
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map);

    // Add a legend to the map
    const legend = L.control({position: 'bottomright'});

    legend.onAdd = function(map) {
        const div = L.DomUtil.create('div', 'map-legend');
        div.innerHTML = `
            <h4>Legend</h4>
            <div class="legend-item">
                <div class="legend-color blue"></div>
                <span>Existing Routes</span>
            </div>
            <div class="legend-item">
                <div class="legend-color green"></div>
                <span>Suggested Routes (Streets)</span>
            </div>
            <div class="legend-item">
                <div class="legend-color orange"></div>
                <span>Suggested Routes (Direct)</span>
            </div>
        `;
        return div;
    };

    legend.addTo(map);

    // Layer groups for different route types
    const existingRoutesLayer = L.layerGroup().addTo(map);
    const suggestedRoutesLayer = L.layerGroup().addTo(map);

    // DOM elements
    const uploadForm = document.getElementById('upload-form');
    const uploadStatus = document.getElementById('upload-status');
    const suggestButton = document.getElementById('suggest-button');
    const showExistingButton = document.getElementById('show-existing-button');
    const clearMapButton = document.getElementById('clear-map-button');
    const minDistanceInput = document.getElementById('min-distance');
    const maxDistanceInput = document.getElementById('max-distance');
    const followStreetsCheckbox = document.getElementById('follow-streets');

    // Handle file upload
    uploadForm.addEventListener('submit', function(e) {
        e.preventDefault();

        const formData = new FormData(uploadForm);
        const fileInput = document.getElementById('gpx-file');

        if (fileInput.files.length === 0) {
            showStatus('Please select a GPX file to upload', 'error');
            return;
        }

        // Show loading status
        showStatus('Uploading file...', '');

        fetch('/upload', {
            method: 'POST',
            body: formData
        })
        .then(response => {
            if (!response.ok) {
                throw new Error('Upload failed');
            }
            return response.json();
        })
        .then(data => {
            showStatus(data.message, 'success');
            // Refresh the map with existing routes
            loadExistingRoutes();
        })
        .catch(error => {
            showStatus('Error: ' + error.message, 'error');
        });
    });

    // Load existing routes
    function loadExistingRoutes() {
        existingRoutesLayer.clearLayers();

        fetch('/routes')
        .then(response => response.json())
        .then(routes => {
            if (routes.length === 0) {
                showStatus('No existing routes found', '');
                return;
            }

            let bounds = L.latLngBounds();

            routes.forEach(route => {
                if (route.trackPoints && route.trackPoints.length > 0) {
                    const points = route.trackPoints.map(point => [point.lat, point.lng]);
                    const polyline = L.polyline(points, {
                        color: 'blue',
                        weight: 3,
                        className: 'existing-route'
                    });

                    // Calculate route distance if not provided
                    let routeDistance = route.distance;
                    if (!routeDistance || routeDistance === 0) {
                        routeDistance = calculateRouteDistance(points);
                    }

                    polyline.bindPopup(`
                        <strong>${route.filename}</strong><br>
                        Distance: ${routeDistance.toFixed(2)} km<br>
                        Duration: ${formatDuration(route.duration)}
                    `);

                    // Show distance when hovering over the route
                    polyline.bindTooltip(`${routeDistance.toFixed(2)} km`, {
                        permanent: false,
                        direction: 'center',
                        className: 'route-tooltip'
                    });

                    existingRoutesLayer.addLayer(polyline);
                    bounds.extend(polyline.getBounds());
                }
            });

            if (!bounds.isValid()) {
                return;
            }

            map.fitBounds(bounds);
            showStatus(`Loaded ${routes.length} routes`, 'success');
        })
        .catch(error => {
            showStatus('Error loading routes: ' + error.message, 'error');
        });
    }

    // Show existing routes button
    showExistingButton.addEventListener('click', function() {
        loadExistingRoutes();
    });

    // Suggest new routes
    suggestButton.addEventListener('click', function() {
        suggestedRoutesLayer.clearLayers();

        let minDistance = minDistanceInput.value ? parseFloat(minDistanceInput.value) : 0;
        let maxDistance = maxDistanceInput.value ? parseFloat(maxDistanceInput.value) : 0;
        const followStreets = followStreetsCheckbox.checked;

        // Validate min/max distance values
        if (minDistance < 0) minDistance = 0;
        if (maxDistance < 0) maxDistance = 0;
        if (maxDistance > 0 && minDistance > maxDistance) {
            // Swap values if min > max
            const temp = minDistance;
            minDistance = maxDistance;
            maxDistance = temp;

            // Update the input fields
            minDistanceInput.value = minDistance;
            maxDistanceInput.value = maxDistance;
        }

        let url = '/suggest';
        const params = [];

        if (minDistance > 0) {
            params.push(`minDistance=${minDistance}`);
        }

        if (maxDistance > 0) {
            params.push(`maxDistance=${maxDistance}`);
        }

        // Only add followStreets parameter if it's false (since true is the default)
        if (!followStreets) {
            params.push(`followStreets=false`);
        }

        if (params.length > 0) {
            url += '?' + params.join('&');
        }

        fetch(url)
        .then(response => response.json())
        .then(routes => {
            if (routes.length === 0) {
                showStatus('No suggested routes found with the current criteria', '');
                return;
            }

            let bounds = L.latLngBounds();

            routes.forEach((route, index) => {
                if (route.points && route.points.length > 0) {
                    const points = route.points.map(point => [point.lat, point.lng]);

                    // Set color and class based on whether the route follows streets
                    const routeColor = route.followsStreets ? 'green' : 'orange';
                    const routeClass = route.followsStreets ? 'suggested-route-streets' : 'suggested-route-direct';
                    const routeType = route.followsStreets ? 'Streets' : 'Direct';

                    // Calculate route distance if not provided or is zero
                    let routeDistance = route.distance;
                    if (!routeDistance || routeDistance === 0) {
                        routeDistance = calculateRouteDistance(points);
                    }
                    const routeDistanceFormatted = routeDistance.toFixed(2);

                    const polyline = L.polyline(points, {
                        color: routeColor,
                        weight: 4,
                        className: routeClass
                    });

                    // Add a popup with route information
                    polyline.bindPopup(`<strong>Route Length:</strong> ${routeDistanceFormatted} km<br><strong>Type:</strong> ${routeType}`);

                    // Show distance when hovering over the route
                    polyline.bindTooltip(`${routeDistanceFormatted} km`, {
                        permanent: false,
                        direction: 'center',
                        className: 'route-tooltip'
                    });

                    polyline.bindPopup(`
                        <strong>Suggested Route ${index + 1}</strong><br>
                        Distance: ${routeDistanceFormatted} km<br>
                        Follows Streets: ${route.followsStreets ? 'Yes' : 'No'}
                    `);

                    suggestedRoutesLayer.addLayer(polyline);
                    bounds.extend(polyline.getBounds());
                }
            });

            if (!bounds.isValid()) {
                return;
            }

            map.fitBounds(bounds);

            // Display route information in the status message
            const routeInfo = routes.map((route, index) => {
                // Calculate route distance if not provided or is zero
                let routeDistance = route.distance;
                if (!routeDistance || routeDistance === 0) {
                    const points = route.points.map(point => [point.lat, point.lng]);
                    routeDistance = calculateRouteDistance(points);
                }
                return `Route ${index + 1}: ${routeDistance.toFixed(2)} km (${route.followsStreets ? 'Streets' : 'Direct'})`;
            }).join(' | ');

            showStatus(`Found ${routes.length} suggested routes: ${routeInfo}`, 'success');

            // Add a route info panel to the map
            if (!document.getElementById('route-info-panel')) {
                const routeInfoPanel = L.control({position: 'topright'});

                routeInfoPanel.onAdd = function(map) {
                    const div = L.DomUtil.create('div', 'route-info-panel');
                    div.id = 'route-info-panel';
                    return div;
                };

                routeInfoPanel.addTo(map);
            }

            // Update the route info panel
            const routeInfoPanel = document.getElementById('route-info-panel');
            routeInfoPanel.innerHTML = `
                <h4>Suggested Routes</h4>
                ${routes.map((route, index) => {
                    // Calculate route distance if not provided or is zero
                    let routeDistance = route.distance;
                    if (!routeDistance || routeDistance === 0) {
                        const points = route.points.map(point => [point.lat, point.lng]);
                        routeDistance = calculateRouteDistance(points);
                    }
                    return `<div class="route-info-item">
                        <div class="route-color ${route.followsStreets ? 'green' : 'orange'}"></div>
                        <span>Route ${index + 1}: ${routeDistance.toFixed(2)} km</span>
                    </div>`;
                }).join('')}
            `;
        })
        .catch(error => {
            showStatus('Error suggesting routes: ' + error.message, 'error');
        });
    });

    // Clear map button
    clearMapButton.addEventListener('click', function() {
        existingRoutesLayer.clearLayers();
        suggestedRoutesLayer.clearLayers();
        showStatus('Map cleared', '');
    });

    // Helper function to show status messages
    function showStatus(message, type) {
        uploadStatus.textContent = message;
        uploadStatus.className = type;
    }

    // Helper function to format duration in seconds to a readable format
    function formatDuration(seconds) {
        if (!seconds) return 'N/A';

        const hours = Math.floor(seconds / 3600);
        const minutes = Math.floor((seconds % 3600) / 60);
        const secs = Math.floor(seconds % 60);

        let result = '';
        if (hours > 0) {
            result += `${hours}h `;
        }
        if (minutes > 0 || hours > 0) {
            result += `${minutes}m `;
        }
        result += `${secs}s`;

        return result;
    }

    // Helper function to calculate route distance from points
    function calculateRouteDistance(points) {
        if (!points || points.length < 2) return 0;

        let distance = 0;
        for (let i = 0; i < points.length - 1; i++) {
            distance += haversineDistance(
                points[i][0], points[i][1],
                points[i+1][0], points[i+1][1]
            );
        }
        return distance;
    }

    // Haversine formula to calculate distance between two points
    function haversineDistance(lat1, lon1, lat2, lon2) {
        // Earth's radius in kilometers
        const R = 6371.0;

        // Convert degrees to radians
        const lat1Rad = lat1 * (Math.PI / 180);
        const lat2Rad = lat2 * (Math.PI / 180);
        const lonDiff = (lon2 - lon1) * (Math.PI / 180);

        // Haversine formula
        const a = (1 - Math.cos(lat2Rad - lat1Rad)) / 2 +
                  Math.cos(lat1Rad) * Math.cos(lat2Rad) * (1 - Math.cos(lonDiff)) / 2;
        const distance = 2 * R * Math.asin(Math.sqrt(a));

        return distance;
    }

    // Try to load existing routes on page load
    loadExistingRoutes();
});
