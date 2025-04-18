// Walk Assistant JavaScript

document.addEventListener('DOMContentLoaded', function() {
    // Initialize map
    const map = L.map('map-container').setView([0, 0], 13);
    
    // Add OpenStreetMap tiles
    L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
    }).addTo(map);
    
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
                    const polyline = L.polyline(points, { color: 'blue', weight: 3 });
                    
                    polyline.bindPopup(`
                        <strong>${route.filename}</strong><br>
                        Distance: ${(route.distance).toFixed(2)} km<br>
                        Duration: ${formatDuration(route.duration)}
                    `);
                    
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
        
        const minDistance = minDistanceInput.value ? parseFloat(minDistanceInput.value) : 0;
        const maxDistance = maxDistanceInput.value ? parseFloat(maxDistanceInput.value) : 0;
        
        let url = '/suggest';
        const params = [];
        
        if (minDistance > 0) {
            params.push(`minDistance=${minDistance}`);
        }
        
        if (maxDistance > 0) {
            params.push(`maxDistance=${maxDistance}`);
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
                    const polyline = L.polyline(points, { color: 'green', weight: 4 });
                    
                    polyline.bindPopup(`
                        <strong>Suggested Route ${index + 1}</strong><br>
                        Distance: ${(route.distance).toFixed(2)} km
                    `);
                    
                    suggestedRoutesLayer.addLayer(polyline);
                    bounds.extend(polyline.getBounds());
                }
            });
            
            if (!bounds.isValid()) {
                return;
            }
            
            map.fitBounds(bounds);
            showStatus(`Found ${routes.length} suggested routes`, 'success');
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
    
    // Try to load existing routes on page load
    loadExistingRoutes();
});
