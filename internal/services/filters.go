package services

import (
    "strings"

    "github.com/Zerr0-C00L/StreamArr/internal/models"
)

// IsIndianMovie returns true if the movie appears to be from India
// Uses TMDB metadata: original_language=hi (Hindi) or production_countries includes "IN" (India)
func IsIndianMovie(m *models.Movie) bool {
    if m == nil || m.Metadata == nil {
        return false
    }
    
    // Check original language is Hindi
    if ol, ok := m.Metadata["original_language"].(string); ok {
        if strings.EqualFold(ol, "hi") {
            return true
        }
    }
    
    // Check production countries includes India
    if pcs, ok := m.Metadata["production_countries"].([]string); ok {
        for _, c := range pcs {
            if strings.EqualFold(c, "IN") {
                return true
            }
        }
    } else if arr, ok := m.Metadata["production_countries"].([]interface{}); ok {
        for _, v := range arr {
            if s, ok2 := v.(string); ok2 && strings.EqualFold(s, "IN") {
                return true
            }
        }
    }
    
    // IPTV VOD category heuristics (for manual categorization)
    if cat, ok := m.Metadata["iptv_vod_category"].(string); ok {
        lc := strings.ToLower(cat)
        if strings.Contains(lc, "bollywood") || strings.Contains(lc, "hindi") || strings.Contains(lc, "desi") {
            return true
        }
    }
    
    return false
}

// IsIndianSeries returns true if the series appears to be from India
// Uses TMDB metadata: original_language=hi (Hindi) or origin_country includes "IN" (India)
func IsIndianSeries(s *models.Series) bool {
    if s == nil || s.Metadata == nil {
        return false
    }
    
    // Check original language is Hindi
    if ol, ok := s.Metadata["original_language"].(string); ok {
        if strings.EqualFold(ol, "hi") {
            return true
        }
    }
    
    // Check origin country includes India
    if oc, ok := s.Metadata["origin_country"].([]string); ok {
        for _, c := range oc {
            if strings.EqualFold(c, "IN") {
                return true
            }
        }
    } else if arr, ok := s.Metadata["origin_country"].([]interface{}); ok {
        for _, v := range arr {
            if st, ok2 := v.(string); ok2 && strings.EqualFold(st, "IN") {
                return true
            }
        }
    }
    
    // IPTV VOD category heuristics (for manual categorization)
    if cat, ok := s.Metadata["iptv_vod_category"].(string); ok {
        lc := strings.ToLower(cat)
        if strings.Contains(lc, "bollywood") || strings.Contains(lc, "hindi") || strings.Contains(lc, "desi") {
            return true
        }
    }
    
    return false
}
