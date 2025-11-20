import puppeteer from 'puppeteer';
import pool from './config/database.js';

const BASE_URL = 'https://www.potterybarn.com/shop/sale/open-box-deals/';

async function scrapeProductsWithPuppeteer() {
  let browser;
  try {
    console.log('Launching browser...');
    browser = await puppeteer.launch({
      headless: 'new',
      args: [
        '--no-sandbox',
        '--disable-setuid-sandbox',
        '--disable-dev-shm-usage',
        '--disable-accelerated-2d-canvas',
        '--no-first-run',
        '--no-zygote',
        '--disable-gpu'
      ],
      timeout: 30000
    });
    
    // Wait a bit for browser to be ready
    await new Promise(resolve => setTimeout(resolve, 1000));
    
    const page = await browser.newPage();
    
    // Set viewport
    await page.setViewport({ width: 1920, height: 1080 });
    
    // Set user agent
    await page.setUserAgent('Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36');
    
    console.log(`Navigating to ${BASE_URL}...`);
    await page.goto(BASE_URL, {
      waitUntil: 'networkidle2',
      timeout: 60000
    });
    
    // Wait for initial products to load
    console.log('Waiting for products to load...');
    try {
      await page.waitForSelector('[data-component="Shop-ProductCell"], .grid-item', {
        timeout: 10000
      });
    } catch (e) {
      console.log('Products selector not found immediately, continuing...');
    }
    
    // Scroll to load all products (infinite scroll)
    console.log('Scrolling to load all products...');
    let previousProductCount = 0;
    let scrollAttempts = 0;
    const maxScrollAttempts = 50; // Prevent infinite loops
    let noNewProductsCount = 0;
    
    while (scrollAttempts < maxScrollAttempts) {
      // Get current product count
      const currentProductCount = await page.evaluate(() => {
        return document.querySelectorAll('[data-component="Shop-ProductCell"], .grid-item').length;
      });
      
      console.log(`  Found ${currentProductCount} products so far...`);
      
      // If no new products after 3 scrolls, we're done
      if (currentProductCount === previousProductCount) {
        noNewProductsCount++;
        if (noNewProductsCount >= 3) {
          console.log('No new products loaded, scrolling complete.');
          break;
        }
      } else {
        noNewProductsCount = 0;
      }
      
      previousProductCount = currentProductCount;
      
      // Scroll down
      await page.evaluate(() => {
        window.scrollBy(0, window.innerHeight);
      });
      
      // Wait for new content to load
      await page.waitForTimeout(2000);
      
      scrollAttempts++;
    }
    
    console.log(`Finished scrolling. Total products found: ${previousProductCount}`);
    
    // Extract products from the page
    console.log('Extracting product data...');
    const products = await page.evaluate(() => {
      const productElements = document.querySelectorAll('[data-component="Shop-ProductCell"], .grid-item');
      const products = [];
      const seenUrls = new Set();
      
      productElements.forEach((element) => {
        try {
          // Extract product name
          const nameElement = element.querySelector('[data-test-id="product-info"] span, .product-name a span, .product-name span');
          const name = nameElement ? nameElement.textContent.trim() : null;
          
          // Extract product URL
          const linkElement = element.querySelector('[data-test-id="product-image-link"], .product-image-link, .product-name a');
          let productUrl = linkElement ? linkElement.getAttribute('href') : null;
          
          if (productUrl && !productUrl.startsWith('http')) {
            productUrl = `https://www.potterybarn.com${productUrl}`;
          }
          
          // Skip if we've already seen this URL or no URL
          if (!productUrl || seenUrls.has(productUrl) || !name) {
            return;
          }
          
          seenUrls.add(productUrl);
          
          // Extract image
          const imageElement = element.querySelector('[data-test-id="product-image"], img.product-image, .product-image-link img');
          let image = imageElement ? (imageElement.getAttribute('src') || imageElement.getAttribute('data-src')) : null;
          
          if (image && !image.startsWith('http')) {
            if (image.startsWith('//')) {
              image = `https:${image}`;
            } else {
              image = `https://www.potterybarn.com${image}`;
            }
          }
          
          // Extract price - get all price amounts
          const priceElements = element.querySelectorAll('[data-test-id="amount"], .product-price .amount');
          const prices = [];
          priceElements.forEach(el => {
            const priceText = el.textContent.trim();
            const price = parseFloat(priceText.replace(/,/g, ''));
            if (!isNaN(price)) {
              prices.push(price);
            }
          });
          
          // Use the first price, or calculate range if multiple
          let price = null;
          if (prices.length > 0) {
            if (prices.length === 1) {
              price = prices[0];
            } else {
              // If there's a range, use the minimum (sale price)
              price = Math.min(...prices);
            }
          }
          
          // Extract grade - look for contract grade, open box, etc.
          let grade = null;
          
          // Check for contract grade
          const contractGradeElement = element.querySelector('.contractgrade');
          if (contractGradeElement) {
            grade = 'Contract Grade';
          }
          
          // Check for open box flag
          const openBoxFlag = element.querySelector('.flag-text, [aria-label*="Open Box"], .flagInner');
          if (openBoxFlag && !grade) {
            const flagText = openBoxFlag.textContent.trim().toLowerCase();
            if (flagText.includes('open box')) {
              grade = 'Open Box';
            } else if (flagText.includes('grade a') || flagText.includes('grade-a')) {
              grade = 'A';
            } else if (flagText.includes('grade b') || flagText.includes('grade-b')) {
              grade = 'B';
            } else if (flagText.includes('grade c') || flagText.includes('grade-c')) {
              grade = 'C';
            }
          }
          
          // Check product name and description for grade indicators
          if (!grade) {
            const allText = element.textContent.toLowerCase();
            if (allText.includes('grade a') || allText.includes('grade-a')) {
              grade = 'A';
            } else if (allText.includes('grade b') || allText.includes('grade-b')) {
              grade = 'B';
            } else if (allText.includes('grade c') || allText.includes('grade-c')) {
              grade = 'C';
            } else if (allText.includes('open box')) {
              grade = 'Open Box';
            }
          }
          
          products.push({
            name: name,
            price: price,
            grade: grade,
            image_url: image,
            product_url: productUrl
          });
        } catch (error) {
          console.error('Error extracting product:', error);
        }
      });
      
      return products;
    });
    
    await browser.close();
    return products;
  } catch (error) {
    if (browser) {
      try {
        await browser.close();
      } catch (closeError) {
        // Ignore close errors
      }
    }
    console.error('Error scraping with Puppeteer:', error.message || error);
    throw error;
  }
}

function parsePrice(priceText) {
  if (!priceText) return null;
  
  const cleaned = priceText.replace(/[^\d.,]/g, '');
  const match = cleaned.match(/[\d,]+\.?\d*/);
  if (match) {
    return parseFloat(match[0].replace(/,/g, ''));
  }
  return null;
}

function extractGrade(text, name) {
  if (!text && !name) return null;
  
  const combinedText = `${text || ''} ${name || ''}`.toLowerCase();
  
  if (combinedText.includes('grade a') || combinedText.includes('grade-a') || combinedText.includes('grade: a')) {
    return 'A';
  }
  if (combinedText.includes('grade b') || combinedText.includes('grade-b') || combinedText.includes('grade: b')) {
    return 'B';
  }
  if (combinedText.includes('grade c') || combinedText.includes('grade-c') || combinedText.includes('grade: c')) {
    return 'C';
  }
  if (combinedText.includes('excellent condition') || combinedText.includes('excellent')) {
    return 'A';
  }
  if (combinedText.includes('good condition') || combinedText.includes('good')) {
    return 'B';
  }
  if (combinedText.includes('fair condition') || combinedText.includes('fair')) {
    return 'C';
  }
  if (combinedText.includes('open box')) {
    return 'Open Box';
  }
  if (combinedText.includes('contract grade')) {
    return 'Contract Grade';
  }
  
  return null;
}

async function saveProducts(products) {
  const client = await pool.connect();
  try {
    await client.query('BEGIN');
    
    let saved = 0;
    let updated = 0;
    
    for (const product of products) {
      // Skip if product_url is null (required for unique constraint)
      if (!product.product_url) {
        console.warn('Skipping product without URL:', product.name);
        continue;
      }
      
      const result = await client.query(
        `INSERT INTO products (name, price, grade, image_url, product_url, updated_at)
         VALUES ($1, $2, $3, $4, $5, CURRENT_TIMESTAMP)
         ON CONFLICT (product_url) 
         DO UPDATE SET 
           name = EXCLUDED.name,
           price = EXCLUDED.price,
           grade = EXCLUDED.grade,
           image_url = EXCLUDED.image_url,
           updated_at = CURRENT_TIMESTAMP
         RETURNING id, created_at, updated_at`,
        [product.name, product.price, product.grade, product.image_url, product.product_url]
      );
      
      if (result.rows[0]) {
        const row = result.rows[0];
        // Check if it was an insert or update
        if (row.created_at.getTime() === row.updated_at.getTime()) {
          saved++;
        } else {
          updated++;
        }
      }
    }
    
    await client.query('COMMIT');
    console.log(`Saved ${saved} new products, updated ${updated} existing products`);
    return { saved, updated, total: products.length };
  } catch (error) {
    await client.query('ROLLBACK');
    console.error('Error saving products:', error);
    throw error;
  } finally {
    client.release();
  }
}

async function scrapeProducts() {
  const maxRetries = 3;
  let lastError;
  
  for (let attempt = 1; attempt <= maxRetries; attempt++) {
    try {
      console.log(`Starting scrape (attempt ${attempt}/${maxRetries})...`);
      
      const products = await scrapeProductsWithPuppeteer();
      
      console.log(`Found ${products.length} products`);
      
      if (products.length === 0) {
        console.warn('No products found.');
        return { saved: 0, updated: 0, total: 0 };
      }
      
      const result = await saveProducts(products);
      console.log('Scraping complete:', result);
      return result;
    } catch (error) {
      lastError = error;
      console.error(`Scraping failed (attempt ${attempt}/${maxRetries}):`, error.message || error);
      
      if (attempt < maxRetries) {
        const waitTime = attempt * 2000; // Exponential backoff
        console.log(`Retrying in ${waitTime / 1000} seconds...`);
        await new Promise(resolve => setTimeout(resolve, waitTime));
      }
    }
  }
  
  console.error('Scraping failed after all retries');
  throw lastError;
}

// Run if called directly
const isMainModule = import.meta.url === `file://${process.argv[1]}` || 
                     process.argv[1]?.endsWith('scraper.js') ||
                     process.argv[1]?.includes('scraper.js');

if (isMainModule) {
  scrapeProducts()
    .then(() => {
      console.log('Scraping finished');
      process.exit(0);
    })
    .catch((error) => {
      console.error('Scraping error:', error);
      process.exit(1);
    });
}

export default scrapeProducts;
