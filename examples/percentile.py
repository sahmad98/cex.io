#!/usr/bin/python

import sys
import numpy as np

def main():
    data = []
    percentiles = []
    points = [0, 50, 75, 90, 95, 99, 100]
    for line in sys.stdin:
        data.append(int(line.strip()))
    arr = np.array(data)
    print points
    print np.percentile(arr, points)

if __name__ == '__main__':
    main()
