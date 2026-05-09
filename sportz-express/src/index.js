import express from 'express';

const app = express();
const port = 8000;

app.use(express.json());

app.get('/', (req, res) => {
    res.send('Welcome to Sportz Express API!');
});

app.listen(port, () => {
    console.log(`Server is running on http://localhost:${port}`);
})