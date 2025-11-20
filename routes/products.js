import express from 'express';
import pool from '../config/database.js';
import scrapeProducts from '../scraper.js';

const router = express.Router();

// POST trigger scrape (must come before /:id route)
router.post('/scrape', async (req, res) => {
  try {
    console.log('Scrape endpoint called');
    const result = await scrapeProducts();
    res.json({
      message: 'Scraping completed successfully',
      ...result
    });
  } catch (error) {
    console.error('Error scraping products:', error);
    res.status(500).json({ 
      error: 'Failed to scrape products',
      message: error.message 
    });
  }
});

// GET products by grade (must come before /:id route)
router.get('/grade/:grade', async (req, res) => {
  try {
    const { grade } = req.params;
    const result = await pool.query(
      'SELECT * FROM products WHERE grade = $1 ORDER BY created_at DESC',
      [grade]
    );
    
    res.json({
      products: result.rows,
      count: result.rows.length
    });
  } catch (error) {
    console.error('Error fetching products by grade:', error);
    res.status(500).json({ error: 'Failed to fetch products' });
  }
});

// GET all products (with optional name search)
router.get('/', async (req, res) => {
  try {
    const { page = 1, limit = 50, sort = 'created_at', order = 'DESC', name } = req.query;
    const offset = (page - 1) * limit;
    
    const validSorts = ['created_at', 'name', 'price', 'grade'];
    const validOrders = ['ASC', 'DESC'];
    
    const sortColumn = validSorts.includes(sort) ? sort : 'created_at';
    const sortOrder = validOrders.includes(order.toUpperCase()) ? order.toUpperCase() : 'DESC';
    
    let query, countQuery, queryParams;
    
    // If name search is provided, filter by name
    if (name && name.trim()) {
      query = `SELECT id, name, price, grade, image_url, product_url, created_at, updated_at
               FROM products
               WHERE name ILIKE $1
               ORDER BY ${sortColumn} ${sortOrder}
               LIMIT $2 OFFSET $3`;
      countQuery = 'SELECT COUNT(*) FROM products WHERE name ILIKE $1';
      queryParams = [`%${name}%`, limit, offset];
    } else {
      query = `SELECT id, name, price, grade, image_url, product_url, created_at, updated_at
               FROM products
               ORDER BY ${sortColumn} ${sortOrder}
               LIMIT $1 OFFSET $2`;
      countQuery = 'SELECT COUNT(*) FROM products';
      queryParams = [limit, offset];
    }
    
    const result = await pool.query(query, queryParams);
    
    const countParams = name && name.trim() ? [`%${name}%`] : [];
    const countResult = await pool.query(countQuery, countParams);
    const total = parseInt(countResult.rows[0].count);
    
    res.json({
      products: result.rows,
      pagination: {
        page: parseInt(page),
        limit: parseInt(limit),
        total,
        totalPages: Math.ceil(total / limit)
      }
    });
  } catch (error) {
    console.error('Error fetching products:', error);
    res.status(500).json({ error: 'Failed to fetch products' });
  }
});

// GET product by ID (must be last to avoid catching other routes)
router.get('/:id', async (req, res) => {
  try {
    // Check if the id is actually a number to avoid catching other routes
    const id = parseInt(req.params.id);
    if (isNaN(id)) {
      return res.status(400).json({ error: 'Invalid product ID' });
    }
    
    const result = await pool.query(
      'SELECT * FROM products WHERE id = $1',
      [id]
    );
    
    if (result.rows.length === 0) {
      return res.status(404).json({ error: 'Product not found' });
    }
    
    res.json(result.rows[0]);
  } catch (error) {
    console.error('Error fetching product:', error);
    res.status(500).json({ error: 'Failed to fetch product' });
  }
});

export default router;

