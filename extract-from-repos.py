
#!/usr/bin/env python3
"""
Extract all content from Balkan-On-Demand and DomaciFlix repos
Creates a complete M3U with full TMDB metadata
"""

import json
import re
from urllib.request import urlopen, Request
from urllib.error import URLError

def fetch_json(url):
    """Fetch JSON from URL"""
    try:
        req = Request(url)
        req.add_header('User-Agent', 'StreamArr-Extractor/1.0')
        with urlopen(req, timeout=30) as response:
            return json.loads(response.read())
    except Exception as e:
        print(f"Error fetching {url}: {e}")
        return None

def extract_tmdb_from_poster(poster_url):
    """Extract TMDB ID from poster URL"""
    if not poster_url or 'tmdb.org' not in poster_url:
        return None
    match = re.search(r'/(\d+)\.jpg', poster_url)
    return match.group(1) if match else None

def create_extinf(item, content_type='movie'):
    """Create EXTINF line from item metadata"""
    parts = ['#EXTINF:-1']
    
    # Add TMDB ID
    tmdb_id = None
    if item.get('id') and (item['id'].startswith('tt') or item['id'].isdigit()):
        tmdb_id = item['id'].replace('tt', '')
    elif item.get('poster'):
        tmdb_id = extract_tmdb_from_poster(item['poster'])
    
    if tmdb_id:
        parts.append(f'tvg-id="{tmdb_id}"')
    
    # Add name
    name = item.get('name', 'Unknown')
    parts.append(f'tvg-name="{name}"')
    
    # Add poster
    if item.get('poster'):
        parts.append(f'tvg-logo="{item["poster"]}"')
    
    # Add group-title
    group = 'Series' if content_type == 'series' else 'Movies'
    parts.append(f'group-title="{group}"')
    
    # Build title with year
    title = name
    if item.get('year'):
        title = f"{name} ({item['year']})"
    
    return ' '.join(parts) + f', {title}'

def main():
    print("📥 Extracting content from GitHub repos...")
    print()
    
    all_entries = []
    
    # 1. Balkan-On-Demand (use full backup with all content)
    print("📚 Fetching Balkan-On-Demand content (full catalog)...")
    baubau_url = "https://raw.githubusercontent.com/Zerr0-C00L/Balkan-On-Demand/main/data/baubau-content-full-backup.json"
    baubau_data = fetch_json(baubau_url)
    
    if baubau_data:
        # Only include domestic/Ex-Yu categories
        domestic_categories = [
            'EX YU FILMOVI',
            'EX YU SERIJE',
            'EXYU SERIJE',
            'EXYU SERIJE KOJE SE EMITUJU',
            'KLIK PREMIJERA',
            'KLASICI',
            'FILMSKI KLASICI',
            'Bolji Zivot',
            'Bela Ladja',
            'Policajac Sa Petlovog Brda',
            'Slatke Muke'
        ]
        
        # Movies
        movie_count = 0
        for movie in baubau_data.get('movies', []):
            # Filter by category
            if movie.get('category') not in domestic_categories:
                continue
            
            if movie.get('streams'):
                for stream in movie['streams']:
                    extinf = create_extinf(movie, 'movie')
                    all_entries.append((extinf, stream['url']))
                movie_count += 1
        
        # Series (include all - they don't have categories, they're all domestic)
        series_count = 0
        for series in baubau_data.get('series', []):
            if series.get('streams'):
                for stream in series['streams']:
                    extinf = create_extinf(series, 'series')
                    all_entries.append((extinf, stream['url']))
                series_count += 1
        
        print(f"   ✓ Loaded {movie_count} domestic movies + {series_count} domestic series")
    
    # 2. DomaciFlix - Movies (paginated)
    print("📚 Fetching DomaciFlix movies (all pages)...")
    domaci_movie_count = 0
    skip = 0
    while True:
        domaci_movies_url = f"https://domaci-flixx.vercel.app/catalog/movie/domaci_filmovi/skip={skip}.json"
        domaci_movies = fetch_json(domaci_movies_url)
        
        if not domaci_movies or not domaci_movies.get('metas'):
            break
        
        metas = domaci_movies['metas']
        if not metas:
            break
        
        domaci_movie_count += len(metas)
        print(f"   → Page {skip // 100 + 1}: {len(metas)} movies (total: {domaci_movie_count})")
        
        for meta in metas:
            # We need to get stream URLs - fetch meta details
            meta_id = meta.get('id', '')
            if meta_id:
                try:
                    meta_url = f"https://domaci-flixx.vercel.app/meta/movie/{meta_id}.json"
                    meta_detail = fetch_json(meta_url)
                    
                    if meta_detail and meta_detail.get('meta'):
                        item = meta_detail['meta']
                        
                        # Get streams
                        stream_url = f"https://domaci-flixx.vercel.app/stream/movie/{meta_id}.json"
                        stream_data = fetch_json(stream_url)
                        
                        if stream_data and stream_data.get('streams'):
                            for stream in stream_data['streams']:
                                if stream.get('url'):
                                    extinf = create_extinf(item, 'movie')
                                    all_entries.append((extinf, stream['url']))
                except:
                    pass
        
        skip += 100
    
    print(f"   ✓ Loaded {domaci_movie_count} movies total")
    
    # 3. DomaciFlix - Series (paginated)
    print("📚 Fetching DomaciFlix series (all pages)...")
    domaci_series_count = 0
    skip = 0
    while True:
        domaci_series_url = f"https://domaci-flixx.vercel.app/catalog/series/domaci_serije/skip={skip}.json"
        domaci_series = fetch_json(domaci_series_url)
        
        if not domaci_series or not domaci_series.get('metas'):
            break
        
        metas = domaci_series['metas']
        if not metas:
            break
        
        domaci_series_count += len(metas)
        print(f"   → Page {skip // 100 + 1}: {len(metas)} series (total: {domaci_series_count})")
        
        for meta in metas:
            meta_id = meta.get('id', '')
            if meta_id:
                try:
                    meta_url = f"https://domaci-flixx.vercel.app/meta/series/{meta_id}.json"
                    meta_detail = fetch_json(meta_url)
                    
                    if meta_detail and meta_detail.get('meta'):
                        item = meta_detail['meta']
                        
                        # Get episodes
                        if item.get('videos'):
                            for video in item['videos']:
                                video_id = video.get('id', '')
                                if video_id:
                                    stream_url = f"https://domaci-flixx.vercel.app/stream/series/{video_id}.json"
                                    stream_data = fetch_json(stream_url)
                                    
                                    if stream_data and stream_data.get('streams'):
                                        for stream in stream_data['streams']:
                                            if stream.get('url'):
                                                extinf = create_extinf(item, 'series')
                                                all_entries.append((extinf, stream['url']))
                except:
                    pass
        
        skip += 100
    
    print(f"   ✓ Loaded {domaci_series_count} series total")
    
    # Remove duplicates by URL
    print()
    print("🔄 Removing duplicates...")
    seen_urls = set()
    unique_entries = []
    for extinf, url in all_entries:
        if url not in seen_urls:
            seen_urls.add(url)
            unique_entries.append((extinf, url))
    
    print(f"   ✓ {len(all_entries)} total → {len(unique_entries)} unique entries")
    
    # Create M3U file
    output_file = "channels/balkan_vod_repos.m3u"
    print()
    print(f"💾 Creating M3U file: {output_file}")
    
    with open(output_file, 'w', encoding='utf-8') as f:
        f.write("#EXTM3U\n")
        for extinf, url in unique_entries:
            f.write(f"{extinf}\n")
            f.write(f"{url}\n")
    
    # Count stats
    movies = sum(1 for e, _ in unique_entries if 'group-title="Movies"' in e)
    series = sum(1 for e, _ in unique_entries if 'group-title="Series"' in e)
    with_tmdb = sum(1 for e, _ in unique_entries if 'tvg-id="' in e)
    
    print()
    print("✅ M3U file created successfully!")
    print()
    print("📊 Statistics:")
    print(f"   Total entries: {len(unique_entries)}")
    print(f"   Movies: {movies}")
    print(f"   Series: {series}")
    print(f"   With TMDB IDs: {with_tmdb} ({with_tmdb * 100 // len(unique_entries) if unique_entries else 0}%)")
    print()
    print(f"📝 File: {output_file}")
    print()
    print("💡 Next steps:")
    print("   Import in StreamArr UI: Settings → VOD Import → IPTV/M3U")

if __name__ == '__main__':
    main()
