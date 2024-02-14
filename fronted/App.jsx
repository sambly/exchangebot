import React, { useState, useEffect } from 'react';
import { DataTable } from 'primereact/datatable';
import { Column } from 'primereact/column';
import { ProductService } from './service/ProductService';

export default function BasicDemo() {
    const [products, setProducts] = useState([]);

    useEffect(() => {
        ProductService.getProducts().then(data => setProducts(data));
    }, []);

    // console.log(products);

    let changePrices = JSON.parse(localStorage.getItem('changePrices')) || [];
    let marketsStat = JSON.parse(localStorage.getItem('marketsStat')) || [];
    let favoritePairs = JSON.parse(localStorage.getItem('favoritePairs')) || [];


    
    let data = []
    
    changePrices.forEach((item) => {
        data.push({
            'pair':item,
            'ch3m':item.ch3m,
            'ch15m':item.ch15m,
            'ch1h':item.ch1h,
            'ch4h':item.ch4h,
        });
    });

    console.log(data);
    





    // console.log(changePrices);
    // console.log(marketsStat);
    // console.log(favoritePairs);



    return (
        <div className="card">
            <DataTable value={products} tableStyle={{ minWidth: '50rem' }}>
                <Column field="code" header="Code"></Column>
                <Column field="name" header="Name"></Column>
                <Column field="category" header="Category"></Column>
                <Column field="quantity" header="Quantity"></Column>
            </DataTable>
        </div>
    );
}