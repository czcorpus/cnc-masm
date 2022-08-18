#!/usr/bin/env python3
import json
import sys
from typing import TypedDict, List
import pulp

class SolveData(TypedDict):
    A: List[List[float]]
    b: List[float]

buff = ''
for line in sys.stdin:
    buff += line

data: SolveData = json.loads(buff)
A = data['A']
b = data['b']
num_texts = len(A[0])
num_conditions = len(b)

x_min = 0
x_max = 1

x = pulp.LpVariable.dicts('x', list(range(num_texts)), x_min, x_max)
lp_prob = pulp.LpProblem('Minmax_Problem', pulp.LpMaximize)
lp_prob += pulp.lpSum(x), 'Minimize_the_maximum'
for i in range(num_conditions):
    label = f'Max_constraint_{i}'
    condition = pulp.lpSum([A[i][j]*x[j] for j in range(num_texts)]) <= b[i]
    lp_prob += condition, label

stat = lp_prob.solve(pulp.PULP_CBC_CMD(msg=0))

variables = [0] * len(x)
for idx, lpvar in x.items():
    variables[idx] = lpvar.varValue
print(json.dumps(variables), end='')

