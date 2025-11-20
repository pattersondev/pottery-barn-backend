package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
	_ "github.com/lib/pq"
)

const (
	baseURL = "https://www.potterybarn.com/shop/sale/open-box-deals/"
	dbURL   = "postgresql://potterybarn_user:07qYYLJ81qN8uumJIj8p0fGxZ4P59a1w@dpg-d4ficjqli9vc73ag07qg-a.ohio-postgres.render.com/potterybarn?sslmode=require"
)

type Product struct {
	Name       string   `json:"name"`
	Price      *float64 `json:"price"`
	Grade      *string  `json:"grade"`
	ImageURL   *string  `json:"image"`
	ProductURL string   `json:"url"`
}

func main() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	log.Println("Starting scrape...")
	products, err := scrapeProducts(ctx)
	if err != nil {
		log.Fatalf("Scraping failed: %v", err)
	}

	log.Printf("Found %d products", len(products))

	if len(products) == 0 {
		log.Println("No products found")
		return
	}

	if err := saveProducts(products); err != nil {
		log.Fatalf("Failed to save products: %v", err)
	}

	log.Println("Scraping complete!")
}

func scrapeProducts(ctx context.Context) ([]Product, error) {
	log.Println("Launching browser...")

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("headless-new", true), // Use new headless mode
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.Flag("log-level", "3"), // Suppress chromedp warnings
	)

	allocCtx, cancel := chromedp.NewExecAllocator(ctx, opts...)
	defer cancel()

	ctx, cancel = chromedp.NewContext(allocCtx)
	defer cancel()

	log.Printf("Navigating to %s...", baseURL)
	if err := chromedp.Run(ctx,
		chromedp.Navigate(baseURL),
		chromedp.WaitReady("body"),
	); err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	// Set viewport
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`window.resizeTo(1920, 1080)`, nil),
	); err != nil {
		log.Printf("Warning: failed to set viewport: %v", err)
	}

	log.Println("Waiting for products to load...")
	time.Sleep(2 * time.Second)

	// Wait for initial products
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`[data-component="Shop-ProductCell"], .grid-item`, chromedp.ByQuery),
	); err != nil {
		log.Printf("Warning: products selector not found immediately: %v", err)
	}

	log.Println("Scrolling to load all products...")
	previousCount := 0
	previousHeight := 0
	scrollAttempts := 0
	maxScrollAttempts := 500
	noNewProductsCount := 0
	noHeightChangeCount := 0

	for scrollAttempts < maxScrollAttempts {
		// Get current page height and product count before scrolling
		var currentHeight int
		var currentCountBefore int
		chromedp.Run(ctx,
			chromedp.Evaluate(`document.body.scrollHeight`, &currentHeight),
			chromedp.Evaluate(`document.querySelectorAll('[data-component="Shop-ProductCell"], .grid-item').length`, &currentCountBefore),
		)

		// Scroll to trigger intersection observers - use multiple scroll strategies
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				(function() {
					// Strategy 1: Scroll window to bottom
					const currentScroll = window.pageYOffset || document.documentElement.scrollTop;
					const maxScroll = document.body.scrollHeight - window.innerHeight;
					const scrollAmount = window.innerHeight * 0.9;
					
					// Scroll down
					if (maxScroll - currentScroll < scrollAmount) {
						window.scrollTo(0, document.body.scrollHeight);
					} else {
						window.scrollBy(0, scrollAmount);
					}
					
					// Strategy 2: Find the product container and scroll it if it's scrollable
					const container = document.querySelector('#subcat-page > div.sub-cat-container.sub-cat-container-page-with-aside > section > div.two-column-grid');
					if (container && container.scrollHeight > container.clientHeight) {
						container.scrollTop = container.scrollHeight;
					}
					
					// Strategy 3: Trigger intersection observer by scrolling to last product
					const products = document.querySelectorAll('[data-component="Shop-ProductCell"], .grid-item');
					if (products.length > 0) {
						const lastProduct = products[products.length - 1];
						lastProduct.scrollIntoView({ behavior: 'smooth', block: 'end' });
					}
					
					// Trigger all scroll events
					window.dispatchEvent(new Event('scroll', { bubbles: true }));
					document.dispatchEvent(new Event('scroll', { bubbles: true }));
					
					// Also trigger intersection observer manually by dispatching on last element
					if (products.length > 0) {
						const lastProduct = products[products.length - 1];
						const event = new Event('intersect', { bubbles: true });
						lastProduct.dispatchEvent(event);
					}
				})()
			`, nil),
		); err != nil {
			log.Printf("Error scrolling: %v", err)
			break
		}

		// Wait for scroll animation and content loading
		time.Sleep(1 * time.Second)

		// Check for and click "Show More" or similar buttons
		var buttonClicked bool
		chromedp.Run(ctx,
			chromedp.Evaluate(`
				(function() {
					// Look for various "show more" button patterns
					const buttonSelectors = [
						'button:contains("Show More")',
						'button:contains("Show me more")',
						'button:contains("Load More")',
						'a:contains("Show More")',
						'a:contains("Show me more")',
						'[data-test-id*="show-more"]',
						'[data-test-id*="load-more"]',
						'[class*="show-more"]',
						'[class*="load-more"]',
						'button[aria-label*="more"]',
						'button[aria-label*="More"]'
					];
					
					// Try to find button by text content
					const allButtons = Array.from(document.querySelectorAll('button, a'));
					for (const btn of allButtons) {
						const text = btn.textContent.toLowerCase().trim();
						if (text.includes('show more') || text.includes('show me more') || 
						    text.includes('load more') || text.includes('see more')) {
							btn.click();
							return true;
						}
					}
					
					// Try data attributes
					const dataButtons = document.querySelectorAll('[data-test-id*="more"], [data-test-id*="More"]');
					if (dataButtons.length > 0) {
						dataButtons[0].click();
						return true;
					}
					
					return false;
				})()
			`, &buttonClicked),
		)

		if buttonClicked {
			log.Println("  ✓ Clicked 'Show More' button")
			time.Sleep(3 * time.Second) // Wait for button click to load content
		}

		// Wait for network requests and DOM updates
		time.Sleep(4 * time.Second)

		// Get current product count and page height after scroll
		// Use the specific container selector as well
		var currentCount int
		var newHeight int
		var containerCount int
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('[data-component="Shop-ProductCell"], .grid-item').length`, &currentCount),
			chromedp.Evaluate(`document.body.scrollHeight`, &newHeight),
			chromedp.Evaluate(`document.querySelectorAll('#subcat-page > div.sub-cat-container.sub-cat-container-page-with-aside > section > div.two-column-grid > div.grid-item').length`, &containerCount),
		); err != nil {
			log.Printf("Error getting product count: %v", err)
			break
		}

		// Use the higher count (container might have more)
		if containerCount > currentCount {
			currentCount = containerCount
		}

		log.Printf("  Found %d products so far (page height: %d, change: %d)...", currentCount, newHeight, newHeight-currentHeight)

		// Check if page height changed (indicates new content loaded)
		if newHeight == previousHeight {
			noHeightChangeCount++
		} else {
			noHeightChangeCount = 0
		}

		if currentCount == previousCount {
			noNewProductsCount++
			// Only break if we've tried many times AND haven't seen a button to click
			if noNewProductsCount >= 15 && noHeightChangeCount >= 8 {
				// One last check for buttons before giving up
				var hasButton bool
				chromedp.Run(ctx,
					chromedp.Evaluate(`
						(function() {
							const allButtons = Array.from(document.querySelectorAll('button, a'));
							for (const btn of allButtons) {
								const text = btn.textContent.toLowerCase().trim();
								if (text.includes('show more') || text.includes('show me more') || 
								    text.includes('load more') || text.includes('see more')) {
									return true;
								}
							}
							return false;
						})()
					`, &hasButton),
				)
				if !hasButton {
					log.Println("No new products or page growth detected, and no more buttons found. Scrolling complete.")
					break
				} else {
					log.Println("No new products but found a button, continuing...")
					noNewProductsCount = 0 // Reset counter if button found
				}
			}
		} else {
			noNewProductsCount = 0
			log.Printf("  ✓ New products detected! Count increased from %d to %d", previousCount, currentCount)
		}

		previousCount = currentCount
		previousHeight = newHeight
		scrollAttempts++
	}

	log.Printf("Finished scrolling. Total products found: %d", previousCount)

	// Final aggressive scroll attempts to ensure everything is loaded
	log.Println("Final scroll attempts to ensure all products are loaded...")
	for i := 0; i < 10; i++ {
		// Check for and click buttons first
		var buttonClicked bool
		chromedp.Run(ctx,
			chromedp.Evaluate(`
				(function() {
					const allButtons = Array.from(document.querySelectorAll('button, a'));
					for (const btn of allButtons) {
						const text = btn.textContent.toLowerCase().trim();
						if (text.includes('show more') || text.includes('show me more') || 
						    text.includes('load more') || text.includes('see more')) {
							btn.click();
							return true;
						}
					}
					return false;
				})()
			`, &buttonClicked),
		)
		if buttonClicked {
			log.Printf("  Clicked 'Show More' button in final attempt %d", i+1)
			time.Sleep(4 * time.Second)
		}

		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`
				(function() {
					window.scrollTo(0, document.body.scrollHeight);
					const products = document.querySelectorAll('[data-component="Shop-ProductCell"], .grid-item');
					if (products.length > 0) {
						products[products.length - 1].scrollIntoView({ behavior: 'smooth', block: 'end' });
					}
					window.dispatchEvent(new Event('scroll', { bubbles: true }));
				})()
			`, nil),
		); err == nil {
			time.Sleep(3 * time.Second)
			// Check final count using both selectors
			var finalCount int
			var containerCount int
			if err := chromedp.Run(ctx,
				chromedp.Evaluate(`document.querySelectorAll('[data-component="Shop-ProductCell"], .grid-item').length`, &finalCount),
				chromedp.Evaluate(`document.querySelectorAll('#subcat-page > div.sub-cat-container.sub-cat-container-page-with-aside > section > div.two-column-grid > div.grid-item').length`, &containerCount),
			); err == nil {
				if containerCount > finalCount {
					finalCount = containerCount
				}
				if finalCount > previousCount {
					log.Printf("Found additional products after final scroll attempt %d: %d total", i+1, finalCount)
					previousCount = finalCount
				}
			}
		}
	}

	log.Println("Extracting product data...")

	var productsJSON string

	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const products = [];
				const seen = new Set();
				// Try both selectors - container-specific and general
				const containerElements = document.querySelectorAll('#subcat-page > div.sub-cat-container.sub-cat-container-page-with-aside > section > div.two-column-grid > div.grid-item');
				const generalElements = document.querySelectorAll('[data-component="Shop-ProductCell"], .grid-item');
				// Use the one with more elements
				const elements = containerElements.length >= generalElements.length ? containerElements : generalElements;
				
				console.log('Processing ' + elements.length + ' elements...');
				
				// Process in batches to avoid timeout - use for loop instead of forEach for better performance
				for (let i = 0; i < elements.length; i++) {
					const element = elements[i];
					if (i % 200 === 0 && i > 0) {
						console.log('Processed ' + i + ' of ' + elements.length + ' elements...');
					}
					try {
						// Skip if this doesn't look like a product cell
						if (!element.querySelector('[data-component="Shop-ProductCell"]') && 
						    !element.classList.contains('grid-item') &&
						    !element.querySelector('.product-cell-container')) {
							continue;
						}
						
						// Get product URL first - this is the most reliable identifier
						let url = null;
						
						// Try aria-product attribute first (most reliable)
						const ariaProduct = element.getAttribute('aria-product') || 
						                    element.querySelector('[data-component="Shop-ProductCell"]')?.getAttribute('aria-product');
						if (ariaProduct) {
							url = '/products/' + ariaProduct + '/';
						}
						
						// Try link elements
						if (!url) {
							const linkEl = element.querySelector('[data-test-id="product-image-link"], .product-image-link, .product-name a, a[href*="/product/"]');
							if (linkEl) {
								url = linkEl.getAttribute('href');
							}
						}
						
						// Try any link with /product/ in href
						if (!url) {
							const anyLink = element.querySelector('a[href*="/product/"]');
							if (anyLink) {
								url = anyLink.getAttribute('href');
							}
						}
						
						if (url && !url.startsWith('http')) {
							url = 'https://www.potterybarn.com' + url;
						}
						
						// Skip if we've seen this URL before
						if (url && seen.has(url)) continue;
						
						// Get product name - try multiple selectors
						let name = null;
						
						// Try aria-labelledby
						const ariaLabelledBy = element.querySelector('[data-component="Shop-ProductCell"]')?.getAttribute('aria-labelledby');
						if (ariaLabelledBy) {
							const labelledEl = document.getElementById(ariaLabelledBy);
							if (labelledEl) name = labelledEl.textContent.trim();
						}
						
						// Try standard name selectors
						if (!name) {
							const nameEl = element.querySelector('[data-test-id="product-info"] span, .product-name a span, .product-name span, h2, h3, h4');
							if (nameEl) {
								name = nameEl.textContent.trim();
							}
						}
						
						// Try aria-label on links
						if (!name) {
							const ariaLabel = element.querySelector('a[href*="/product/"]')?.getAttribute('aria-label');
							if (ariaLabel) name = ariaLabel.trim();
						}
						
						// Try img alt as fallback
						if (!name) {
							const imgAlt = element.querySelector('img')?.getAttribute('alt');
							if (imgAlt) name = imgAlt.trim();
						}
						
						// Try to extract from aria-product if we have it
						if (!name && ariaProduct) {
							// Clean up aria-product to make a readable name
							name = ariaProduct.replace(/-/g, ' ').replace(/\b\w/g, l => l.toUpperCase());
						}
						
						// Skip if no name AND no URL (we need at least one)
						if (!name && !url) continue;
						
						// Use URL as unique identifier, or generate one if missing
						if (!url) {
							if (name) {
								url = 'https://www.potterybarn.com/products/' + encodeURIComponent(name.toLowerCase().replace(/\s+/g, '-').replace(/[^a-z0-9-]/g, '')) + '/';
							} else {
								// Last resort: use element index
								url = 'https://www.potterybarn.com/products/unknown-' + i + '/';
							}
						}
						
						seen.add(url);
						
						// Get image
						const imgEl = element.querySelector('[data-test-id="product-image"], img.product-image, .product-image-link img');
						let image = imgEl ? (imgEl.getAttribute('src') || imgEl.getAttribute('data-src')) : null;
						if (image && !image.startsWith('http')) {
							if (image.startsWith('//')) {
								image = 'https:' + image;
							} else {
								image = 'https://www.potterybarn.com' + image;
							}
						}
						
						// Get price
						const priceEls = element.querySelectorAll('[data-test-id="amount"], .product-price .amount');
						let price = null;
						const prices = [];
						priceEls.forEach(el => {
							const priceText = el.textContent.trim().replace(/,/g, '');
							const p = parseFloat(priceText);
							if (!isNaN(p)) prices.push(p);
						});
						if (prices.length > 0) {
							price = Math.min(...prices);
						}
						
						// Get grade
						let grade = null;
						if (element.querySelector('.contractgrade')) {
							grade = 'Contract Grade';
						} else {
							const text = element.textContent.toLowerCase();
							if (text.includes('grade a') || text.includes('grade-a')) {
								grade = 'A';
							} else if (text.includes('grade b') || text.includes('grade-b')) {
								grade = 'B';
							} else if (text.includes('grade c') || text.includes('grade-c')) {
								grade = 'C';
							} else if (text.includes('open box')) {
								grade = 'Open Box';
							}
						}
						
						products.push({
							name: name,
							url: url,
							image: image,
							price: price,
							grade: grade
						});
					} catch (e) {
						console.error('Error extracting product:', e);
					}
				}
				
				console.log('Extracted ' + products.length + ' products from ' + elements.length + ' elements');
				return JSON.stringify(products);
			})()
		`, &productsJSON),
	); err != nil {
		return nil, fmt.Errorf("failed to extract products: %w", err)
	}

	var products []Product
	if err := json.Unmarshal([]byte(productsJSON), &products); err != nil {
		return nil, fmt.Errorf("failed to parse products JSON: %w", err)
	}

	log.Printf("Extracted %d products from %d DOM elements", len(products), previousCount)

	return products, nil
}

func saveProducts(products []Product) error {
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	saved := 0
	updated := 0

	for _, product := range products {
		if product.ProductURL == "" {
			log.Printf("Skipping product without URL: %s", product.Name)
			continue
		}

		var id int
		var createdAt, updatedAt time.Time
		err := tx.QueryRow(`
			INSERT INTO products (name, price, grade, image_url, product_url, updated_at)
			VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
			ON CONFLICT (product_url) 
			DO UPDATE SET 
				name = EXCLUDED.name,
				price = EXCLUDED.price,
				grade = EXCLUDED.grade,
				image_url = EXCLUDED.image_url,
				updated_at = CURRENT_TIMESTAMP
			RETURNING id, created_at, updated_at
		`, product.Name, product.Price, product.Grade, product.ImageURL, product.ProductURL).Scan(&id, &createdAt, &updatedAt)

		if err != nil {
			log.Printf("Error saving product %s: %v", product.Name, err)
			continue
		}

		if createdAt.Equal(updatedAt) {
			saved++
		} else {
			updated++
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Saved %d new products, updated %d existing products", saved, updated)
	return nil
}
