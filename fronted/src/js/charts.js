import { timeToLocal } from './help.js';

import { createChart } from 'lightweight-charts';


export function lw_charts_volume(container_chart, chartOptions, pair, dataVolume) {

    container_chart.innerHTML = '';
    container_chart.style.position = 'relative';

    const chart = createChart(container_chart, chartOptions);
    chart.applyOptions({
        rightPriceScale: {
            scaleMargins: {
                top: 0.1,
                bottom: 0.1,
            },
        },
    });

    const lineSeries = chart.addLineSeries({ color: '#2962FF' });

    lineSeries.setData(dataVolume);
    chart.timeScale().fitContent();

    // Отображение легенды
    const toolTip = document.createElement('div');
    toolTip.className = 'three-line-legend';
    container_chart.appendChild(toolTip);
    toolTip.style.display = 'block';
    toolTip.style.left = 3 + 'px';
    toolTip.style.top = 3 + 'px';
    toolTip.innerHTML = '<div style="font-size: 24px; margin: 4px 0px; color: #20262E">' + pair + '</div>';



}


export function lw_charts_orders(container_chart, chartOptions, pair, orders, update_cadles) {
    return new Promise((resolve, reject) => {
        container_chart.innerHTML = '';
        container_chart.style.position = 'relative';

        let intervals = ['1m', '3m', '15m', '15m', '1h', '4h', '1d'];
        const switcherElement = createSimpleSwitcher(intervals, intervals[0], syncToInterval);

        const chart = createChart(container_chart, chartOptions);
        container_chart.appendChild(switcherElement);

        // Отображение легенды
        const toolTip = document.createElement('div');
        toolTip.className = 'three-line-legend';
        container_chart.appendChild(toolTip);
        toolTip.style.display = 'block';
        toolTip.style.left = 3 + 'px';
        toolTip.style.top = 3 + 'px';
        toolTip.innerHTML = '<div style="font-size: 24px; margin: 4px 0px; color: #20262E">' + pair.value + '</div>';

        var candleSeries = null;

        function syncToInterval(interval) {
            if (candleSeries) {
                chart.removeSeries(candleSeries);
                candleSeries = null;
            }

            candleSeries = chart.addCandlestickSeries();

            update_cadles(pair.value, interval).then(candles => {
                candleSeries.setData(candles);

                // Отображение ордеров на графике
                let markers_chart = [];

                for (let order of orders) {

                    if (order.Pair === pair.value) {

                        let timeCreated = timeToLocal(new Date(order.TimeCreated) / 1000);
                        let timeFinished = timeToLocal(new Date(order.Time) / 1000);

                        if (order.Side == 'BUY') {
                            if (order.Status != 'Close') {
                                markers_chart.push({ time: timeCreated, position: 'belowBar', color: '#00ff00', shape: 'arrowUp', text: `long ${order.ID}` });
                            } else {
                                markers_chart.push({ time: timeCreated, position: 'belowBar', color: '#00ff00', shape: 'arrowUp', text: `long ${order.ID}` });
                                markers_chart.push({ time: timeFinished, position: 'aboveBar', color: '#e91e63', shape: 'arrowDown', text: `long ${order.ID}` });
                            }
                        }
                        if (order.Side == 'SELL') {
                            if (order.Status != 'Close') {
                                markers_chart.push({ time: timeCreated, position: 'belowBar', color: '#00ff00', shape: 'arrowDown', text: `short ${order.ID}` });
                            } else {
                                markers_chart.push({ time: timeCreated, position: 'belowBar', color: '#00ff00', shape: 'arrowDown', text: `short ${order.ID}` });
                                markers_chart.push({ time: timeFinished, position: 'aboveBar', color: '#e91e63', shape: 'arrowUp', text: `short ${order.ID}` });
                            }
                        }
                    }
                }

                candleSeries.setMarkers(markers_chart);
                resolve();
            }).catch(error => {
                reject(error);
            });
        }

        syncToInterval(intervals[0]);
    });
}



export function widget_charts(container_chart, pair) {

    localStorage.setItem('widgetPair', pair);
    let chartWidth = container_chart.clientWidth;
    new TradingView.widget(
        {
            "height": "532",
            width: chartWidth,
            "symbol": "BINANCE:" + pair,
            "interval": "15",
            "timezone": "Europe/Moscow",
            "theme": "Light",
            "style": "1",
            "locale": "ru",
            "toolbar_bg": "#f1f3f6",
            "enable_publishing": false,
            "allow_symbol_change": true,
            "container_id": "tradingview_3418f"
        }
    );
}



function createSimpleSwitcher(items, activeItem, activeItemChangedCallback) {
    var switcherElement = document.createElement('div');
    switcherElement.classList.add('switcher');

    var intervalElements = items.map(function (item) {
        var itemEl = document.createElement('button');
        itemEl.innerText = item;
        itemEl.classList.add('switcher-item');
        itemEl.classList.toggle('switcher-active-item', item === activeItem);
        itemEl.addEventListener('click', function () {
            onItemClicked(item);
        });
        switcherElement.appendChild(itemEl);
        return itemEl;
    });

    function onItemClicked(item) {
        if (item === activeItem) {
            return;
        }

        intervalElements.forEach(function (element, index) {
            element.classList.toggle('switcher-active-item', items[index] === item);
        });

        activeItem = item;

        activeItemChangedCallback(item);
    }

    return switcherElement;
}