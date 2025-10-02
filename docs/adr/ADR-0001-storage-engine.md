# ADR-0001: Storage Engine (BoltDB vs. LevelDB)

- Status: Proposed
- Data: 2025-09-22
- Autor: Daniel Carvalho

## Contexto

Necessidade de definir uma ferramenta de persistência de dados, temos duas opções, BoldDB e LevelDB.


## Decisão

Optou-se pela BoltDB pela simplicidade do uso nesse momento, e considerando uma ferramenta com foco em leitura maior que escrita. Porém, a biblioteca foi arquivada por chegar ao seu 'feature complete', na documentação é indicado  o uso da 'bbolt', um fork da BoltDB, mais moderna e robusta, logo optamos por ela.

## Alternativas Consideradas

- BoltDB
- LevelDB
- bbolt

## Consequências

- Positivas: Simplicidade de implementação; Confiabilidade; Leitura > Escrita 
- Negativas: Leitura > Escrita; pode ser necessário trocar no futuro;


## Implicações de Implementação

- Mudanças em código
